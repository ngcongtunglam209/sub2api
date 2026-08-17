package service

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// addonExpiryLeaderLockKey keeps one instance doing the sweep. Several
	// instances deleting the same rows is harmless, but each would also fire
	// its own cache invalidations for the same users.
	addonExpiryLeaderLockKey = "addon:expiry:leader"
	// addonExpiryLeaderLockTTL bounds crash recovery; a sweep pages through
	// batches, so keep it well above one cycle.
	addonExpiryLeaderLockTTL = 5 * time.Minute
	// addonExpiryBatchSize caps one DELETE.
	addonExpiryBatchSize = 500
	// addonExpiryMaxBatchesPerCycle stops one cycle from monopolising the DB
	// if a large backlog exists. The rest is picked up next tick.
	addonExpiryMaxBatchesPerCycle = 20
)

// AddonExpiryService drops lapsed add-on rows.
//
// Housekeeping, not enforcement. AddonService.ResolveActiveAddons already
// treats a lapsed add-on as gone, so this sweep is not what stops the perk —
// it is what stops the table growing a row per expired purchase forever, and
// what nudges cached auth snapshots to be rebuilt promptly rather than at the
// end of their TTL.
//
// That ordering matters: if correctness depended on the sweep, a stalled
// leader lock or a disabled worker would keep handing out concurrency nobody
// is paying for.
type AddonExpiryService struct {
	repo        UserAddonRepository
	invalidator APIKeyAuthCacheInvalidator
	interval    time.Duration
	stopCh      chan struct{}
	stopOnce    sync.Once
	wg          sync.WaitGroup

	lockCache  LeaderLockCache
	db         *sql.DB
	instanceID string
}

func NewAddonExpiryService(repo UserAddonRepository, interval time.Duration) *AddonExpiryService {
	return &AddonExpiryService{
		repo:       repo,
		interval:   interval,
		stopCh:     make(chan struct{}),
		instanceID: uuid.NewString(),
	}
}

// SetLeaderLock injects the leader-lock cache and DB used to elect a single
// sweeping instance. With both nil the sweep runs ungated (single instance and
// tests).
func (s *AddonExpiryService) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = lockCache
	s.db = db
}

// SetAuthCacheInvalidator lets an expired add-on take effect on cached auth
// snapshots, which carry the purchased concurrency and RPM.
func (s *AddonExpiryService) SetAuthCacheInvalidator(invalidator APIKeyAuthCacheInvalidator) {
	if s == nil {
		return
	}
	s.invalidator = invalidator
}

func (s *AddonExpiryService) Start() {
	if s == nil || s.repo == nil || s.interval <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		s.RunOnce()
		for {
			select {
			case <-ticker.C:
				s.RunOnce()
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *AddonExpiryService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

// RunOnce sweeps one cycle. Exported so tests and admin tooling can trigger it
// without waiting for the ticker.
func (s *AddonExpiryService) RunOnce() {
	if s == nil || s.repo == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	release, ok := tryAcquireSingletonLeaderLock(ctx, s.lockCache, s.db, addonExpiryLeaderLockKey, s.instanceID, addonExpiryLeaderLockTTL)
	if !ok {
		return
	}
	defer release()

	total := 0
	for batch := 0; batch < addonExpiryMaxBatchesPerCycle; batch++ {
		now := time.Now()
		ids, err := s.repo.ListExpiredAddonUserIDs(ctx, now, addonExpiryBatchSize)
		if err != nil {
			log.Printf("[AddonExpiry] List expired add-ons failed: %v", err)
			return
		}
		if len(ids) == 0 {
			break
		}
		deleted, err := s.repo.DeleteExpiredAddons(ctx, now, ids)
		if err != nil {
			log.Printf("[AddonExpiry] Delete expired add-ons failed: %v", err)
			return
		}
		total += deleted
		// Invalidate after the write: the snapshot must be rebuilt from the
		// cleared state, never from the row still on its way out.
		if s.invalidator != nil {
			for _, id := range ids {
				s.invalidator.InvalidateAuthCacheByUserID(ctx, id)
			}
		}
		if len(ids) < addonExpiryBatchSize {
			break
		}
		// A batch that deleted nothing means every row was filtered out by the
		// DELETE's own expiry guard — a renewal landed in between. Looping
		// again would re-read the same rows forever.
		if deleted == 0 {
			break
		}
	}
	if total > 0 {
		log.Printf("[AddonExpiry] Dropped %d expired add-ons", total)
	}
}
