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
	// vipExpiryLeaderLockKey keeps one instance doing the sweep. Several
	// instances clearing the same rows is harmless, but each would also fire
	// its own cache invalidations for the same users.
	vipExpiryLeaderLockKey = "vip:expiry:leader"
	// vipExpiryLeaderLockTTL bounds crash recovery; a sweep pages through
	// batches, so keep it well above one cycle.
	vipExpiryLeaderLockTTL = 5 * time.Minute
	// vipExpiryBatchSize caps one UPDATE. Expiries arrive spread over time, so
	// a normal cycle clears a handful; the batch only matters the first time it
	// runs against a backlog.
	vipExpiryBatchSize = 500
	// vipExpiryMaxBatchesPerCycle stops one cycle from monopolising the DB if a
	// large backlog exists. The rest is picked up next tick.
	vipExpiryMaxBatchesPerCycle = 20
)

// VIPExpiryService retires lapsed VIP tiers.
//
// Reads already treat a lapsed tier as no tier, so this sweep is not what makes
// the perks stop — it is what resets vip_qualifying_spend, which is how a user
// has to earn the tier again rather than resuming from an old total.
type VIPExpiryService struct {
	repo        VIPExpiryRepository
	invalidator APIKeyAuthCacheInvalidator
	interval    time.Duration
	stopCh      chan struct{}
	stopOnce    sync.Once
	wg          sync.WaitGroup

	lockCache  LeaderLockCache
	db         *sql.DB
	instanceID string
}

func NewVIPExpiryService(repo VIPExpiryRepository, interval time.Duration) *VIPExpiryService {
	return &VIPExpiryService{
		repo:       repo,
		interval:   interval,
		stopCh:     make(chan struct{}),
		instanceID: uuid.NewString(),
	}
}

// SetLeaderLock injects the leader-lock cache and DB used to elect a single
// sweeping instance. With both nil the sweep runs ungated (single instance and
// tests).
func (s *VIPExpiryService) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = lockCache
	s.db = db
}

// SetAuthCacheInvalidator lets an expired tier take effect on cached auth
// snapshots, which carry the tier's concurrency floor.
func (s *VIPExpiryService) SetAuthCacheInvalidator(invalidator APIKeyAuthCacheInvalidator) {
	if s == nil {
		return
	}
	s.invalidator = invalidator
}

func (s *VIPExpiryService) Start() {
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

func (s *VIPExpiryService) Stop() {
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
func (s *VIPExpiryService) RunOnce() {
	if s == nil || s.repo == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	release, ok := tryAcquireSingletonLeaderLock(ctx, s.lockCache, s.db, vipExpiryLeaderLockKey, s.instanceID, vipExpiryLeaderLockTTL)
	if !ok {
		return
	}
	defer release()

	total := 0
	for batch := 0; batch < vipExpiryMaxBatchesPerCycle; batch++ {
		ids, err := s.repo.ListExpiredVIPUserIDs(ctx, time.Now(), vipExpiryBatchSize)
		if err != nil {
			log.Printf("[VIPExpiry] List expired vip tiers failed: %v", err)
			return
		}
		if len(ids) == 0 {
			break
		}
		expired, err := s.repo.ExpireVIPTiers(ctx, ids)
		if err != nil {
			log.Printf("[VIPExpiry] Expire vip tiers failed: %v", err)
			return
		}
		total += expired
		// Invalidate after the write: the snapshot must be rebuilt from the
		// cleared tier, never from the one still on its way out.
		if s.invalidator != nil {
			for _, id := range ids {
				s.invalidator.InvalidateAuthCacheByUserID(ctx, id)
			}
		}
		if len(ids) < vipExpiryBatchSize {
			break
		}
		// A batch that changed nothing means every row was filtered out by the
		// UPDATE's own guard; looping again would re-read the same rows forever.
		if expired == 0 {
			break
		}
	}
	if total > 0 {
		log.Printf("[VIPExpiry] Expired %d vip tiers", total)
	}
}
