//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func ladderTier(level int, minSpend, rate float64) VIPTier {
	return VIPTier{
		ID:             int64(level),
		Level:          level,
		Name:           "VIP",
		MinSpendUSD:    minSpend,
		RateMultiplier: rate,
		Concurrency:    8,
		GraceDays:      60,
		Enabled:        true,
	}
}

// The ladder only works if spending more always costs less per token. An
// inverted rung means a customer is better off having spent less, which is the
// one shape the admin UI must not be able to save.
func TestValidateVIPLadder(t *testing.T) {
	valid := []VIPTier{
		ladderTier(1, 20, 0.95),
		ladderTier(2, 100, 0.90),
		ladderTier(3, 400, 0.82),
		ladderTier(4, 1500, 0.70),
	}
	require.NoError(t, validateVIPLadder(valid))
	require.NoError(t, validateVIPLadder(nil))
	require.NoError(t, validateVIPLadder(valid[:1]))

	// Order of the slice must not matter; the admin API appends the candidate.
	shuffled := []VIPTier{valid[2], valid[0], valid[3], valid[1]}
	require.NoError(t, validateVIPLadder(shuffled))

	higherLevelCostsLess := []VIPTier{ladderTier(1, 100, 0.95), ladderTier(2, 50, 0.90)}
	require.Error(t, validateVIPLadder(higherLevelCostsLess))

	higherLevelBillsMore := []VIPTier{ladderTier(1, 20, 0.80), ladderTier(2, 100, 0.90)}
	require.Error(t, validateVIPLadder(higherLevelBillsMore))

	sameThreshold := []VIPTier{ladderTier(1, 100, 0.95), ladderTier(2, 100, 0.90)}
	require.Error(t, validateVIPLadder(sameThreshold))

	duplicateLevel := []VIPTier{ladderTier(1, 20, 0.95), ladderTier(1, 100, 0.90)}
	require.Error(t, validateVIPLadder(duplicateLevel))
}

func TestValidateVIPTierFields(t *testing.T) {
	base := ladderTier(1, 20, 0.95)
	require.NoError(t, validateVIPTierFields(base))

	// A multiplier above 1 charges a paying customer more than the base rate,
	// and 0 makes every request free. Neither is a tier anyone meant to create.
	tooGenerous := base
	tooGenerous.RateMultiplier = 0
	require.Error(t, validateVIPTierFields(tooGenerous))

	surcharge := base
	surcharge.RateMultiplier = 1.2
	require.Error(t, validateVIPTierFields(surcharge))

	// Zero grace days would expire the tier the instant it is granted.
	noGrace := base
	noGrace.GraceDays = 0
	require.Error(t, validateVIPTierFields(noGrace))

	unnamed := base
	unnamed.Name = "   "
	require.Error(t, validateVIPTierFields(unnamed))

	noLevel := base
	noLevel.Level = 0
	require.Error(t, validateVIPTierFields(noLevel))

	noConcurrency := base
	noConcurrency.Concurrency = 0
	require.Error(t, validateVIPTierFields(noConcurrency))
}

// Update sends the whole tier back to the DB, so an input that omits a field
// must carry the stored value forward rather than zeroing it.
func TestApplyVIPTierInput_LeavesOmittedFieldsAlone(t *testing.T) {
	stored := ladderTier(2, 100, 0.90)
	stored.BadgeColor = "#60a5fa"

	name := "  Gold  "
	got := applyVIPTierInput(stored, VIPTierInput{Name: &name})

	require.Equal(t, "Gold", got.Name, "names are trimmed")
	require.Equal(t, stored.Level, got.Level)
	require.InDelta(t, stored.MinSpendUSD, got.MinSpendUSD, 1e-9)
	require.InDelta(t, stored.RateMultiplier, got.RateMultiplier, 1e-9)
	require.Equal(t, stored.Concurrency, got.Concurrency)
	require.Equal(t, stored.GraceDays, got.GraceDays)
	require.Equal(t, stored.BadgeColor, got.BadgeColor)
	require.Equal(t, stored.Enabled, got.Enabled)

	disabled := false
	got = applyVIPTierInput(stored, VIPTierInput{Enabled: &disabled})
	require.False(t, got.Enabled, "an explicit false must not read as omitted")
}
