//go:build unit

package service

import (
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func vipTier(level int, minSpend, rate float64, concurrency, graceDays int) *dbent.VIPTier {
	return &dbent.VIPTier{
		ID:             int64(level),
		Level:          level,
		MinSpendUsd:    minSpend,
		RateMultiplier: rate,
		Concurrency:    concurrency,
		GraceDays:      graceDays,
	}
}

func seededTiers() []*dbent.VIPTier {
	return []*dbent.VIPTier{
		vipTier(1, 20, 0.95, 8, 60),
		vipTier(2, 100, 0.90, 12, 60),
		vipTier(3, 400, 0.82, 20, 60),
		vipTier(4, 1500, 0.70, 32, 60),
	}
}

func TestSelectVIPTier(t *testing.T) {
	tiers := seededTiers()

	cases := []struct {
		name      string
		spend     float64
		wantLevel int
	}{
		{"below entry threshold", 19.99, 0},
		{"exactly at threshold earns the tier", 20, 1},
		{"between tiers keeps the lower one", 399, 2},
		{"far above the top", 100000, 4},
		{"no spend", 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := selectVIPTier(tiers, tc.spend)
			if tc.wantLevel == 0 {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			require.Equal(t, tc.wantLevel, got.Level)
		})
	}
}

// Tier order in the table is not guaranteed, so grading must not depend on it.
func TestSelectVIPTier_IgnoresSliceOrder(t *testing.T) {
	shuffled := []*dbent.VIPTier{
		vipTier(3, 400, 0.82, 20, 60),
		vipTier(1, 20, 0.95, 8, 60),
		vipTier(4, 1500, 0.70, 32, 60),
		vipTier(2, 100, 0.90, 12, 60),
	}
	got := selectVIPTier(shuffled, 500)
	require.NotNil(t, got)
	require.Equal(t, 3, got.Level, "must pick the highest earned tier, not the first match")
}

func TestVipTierExpiry(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	require.Equal(t, time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC), vipTierExpiry(vipTier(1, 20, 0.95, 8, 60), now))

	// A misconfigured tier must still expire; zero grace days would otherwise
	// mean "already lapsed" and the tier would never take effect at all.
	require.Equal(t, time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC), vipTierExpiry(vipTier(1, 20, 0.95, 8, 0), now))
}
