package service

import (
	"context"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	gocache "github.com/patrickmn/go-cache"
	"golang.org/x/sync/singleflight"
)

// defaultVIPRateCacheTTL keeps the tier lookup off the billing hot path. A
// freshly earned tier therefore takes up to this long to start discounting,
// which is the right trade: tiers change a few times per user per year, the
// multiplier is read on every single request.
const defaultVIPRateCacheTTL = time.Minute

// VIPRateRepository is the optional capability the production user repository
// provides for reading a user's active VIP billing multiplier.
type VIPRateRepository interface {
	// GetVIPRateMultiplier returns the multiplier of the user's active tier,
	// or 1 when they hold none, their tier lapsed, or the tier is gone.
	GetVIPRateMultiplier(ctx context.Context, userID int64) (float64, error)
}

type vipRateResolver struct {
	repo  VIPRateRepository
	cache *gocache.Cache
	ttl   time.Duration
	sf    singleflight.Group
}

func newVIPRateResolver(repo VIPRateRepository, ttl time.Duration) *vipRateResolver {
	if repo == nil {
		return nil
	}
	if ttl <= 0 {
		ttl = defaultVIPRateCacheTTL
	}
	return &vipRateResolver{
		repo:  repo,
		cache: gocache.New(ttl, time.Minute),
		ttl:   ttl,
	}
}

// Resolve returns the user's VIP billing multiplier, or 1 when anything at all
// is unavailable. Billing must never be blocked or inflated by a tier lookup:
// failing open at 1 charges the undiscounted price, which is the same thing
// that happened before tiers existed.
func (r *vipRateResolver) Resolve(ctx context.Context, userID int64) float64 {
	if r == nil || r.repo == nil || userID <= 0 {
		return 1
	}
	key := strconv.FormatInt(userID, 10)
	if r.cache != nil {
		if cached, ok := r.cache.Get(key); ok {
			if multiplier, castOK := cached.(float64); castOK {
				return multiplier
			}
		}
	}

	value, err, _ := r.sf.Do(key, func() (any, error) {
		multiplier, repoErr := r.repo.GetVIPRateMultiplier(ctx, userID)
		if repoErr != nil {
			return nil, repoErr
		}
		if multiplier <= 0 {
			multiplier = 1
		}
		if r.cache != nil {
			r.cache.Set(key, multiplier, r.ttl)
		}
		return multiplier, nil
	})
	if err != nil {
		logger.LegacyPrintf("service.gateway", "get vip rate failed, billing at full price: user=%d err=%v", userID, err)
		return 1
	}
	multiplier, ok := value.(float64)
	if !ok {
		return 1
	}
	return multiplier
}
