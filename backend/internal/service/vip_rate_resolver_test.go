//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	gocache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/require"
)

type stubVIPRateRepo struct {
	multiplier float64
	err        error
	calls      int
}

func (s *stubVIPRateRepo) GetVIPRateMultiplier(_ context.Context, _ int64) (float64, error) {
	s.calls++
	return s.multiplier, s.err
}

func TestVIPRateResolver_CachesAndFailsOpen(t *testing.T) {
	ctx := context.Background()

	repo := &stubVIPRateRepo{multiplier: 0.9}
	resolver := newVIPRateResolver(repo, time.Minute)
	require.InDelta(t, 0.9, resolver.Resolve(ctx, 7), 1e-9)
	require.InDelta(t, 0.9, resolver.Resolve(ctx, 7), 1e-9)
	require.Equal(t, 1, repo.calls, "second read must come from cache, not the DB")

	// Billing must never be blocked or discounted by a broken lookup: full
	// price is the pre-VIP behaviour and the only safe fallback.
	failing := newVIPRateResolver(&stubVIPRateRepo{err: context.DeadlineExceeded}, time.Minute)
	require.InDelta(t, 1, failing.Resolve(ctx, 7), 1e-9)

	// A repository that reports a nonsense multiplier must not zero out billing.
	zeroed := newVIPRateResolver(&stubVIPRateRepo{multiplier: 0}, time.Minute)
	require.InDelta(t, 1, zeroed.Resolve(ctx, 7), 1e-9)

	// No capability wired, or no user: full price, no panic.
	require.InDelta(t, 1, newVIPRateResolver(nil, time.Minute).Resolve(ctx, 7), 1e-9)
	require.InDelta(t, 1, resolver.Resolve(ctx, 0), 1e-9)
}

// The agreed layering: VIP scales the group default, an explicit (user, group)
// override replaces it. Stacking both would bill below the number an admin
// picked on purpose.
func TestGetUserGroupRateMultiplier_VIPScalesDefaultButOverrideWins(t *testing.T) {
	ctx := context.Background()

	withVIP := &GatewayService{vipRateResolver: newVIPRateResolver(&stubVIPRateRepo{multiplier: 0.9}, time.Minute)}
	require.InDelta(t, 1.8, withVIP.getUserGroupRateMultiplier(ctx, 101, 202, 2.0), 1e-9)

	overridden := &GatewayService{
		vipRateResolver:    newVIPRateResolver(&stubVIPRateRepo{multiplier: 0.9}, time.Minute),
		userGroupRateCache: gocache.New(time.Minute, time.Minute),
	}
	overridden.userGroupRateCache.Set("101:202", 1.5, time.Minute)
	require.InDelta(t, 1.5, overridden.getUserGroupRateMultiplier(ctx, 101, 202, 2.0), 1e-9)
}
