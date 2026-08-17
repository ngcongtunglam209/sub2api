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

// The perks have to climb with the price too, or a customer is again better off
// having spent less. This is the shape that shipped unnoticed: tiers granting
// 1, 2 and 3 slots with a fourth granting 32, and later the reverse.
func TestValidateVIPLadderRejectsShrinkingPerks(t *testing.T) {
	withPerks := func(level int, minSpend, rate float64, concurrency, rpm int) VIPTier {
		tier := ladderTier(level, minSpend, rate)
		tier.Concurrency = concurrency
		tier.RPMLimit = rpm
		return tier
	}

	rising := []VIPTier{
		withPerks(1, 20, 0.95, 1, 0),
		withPerks(2, 100, 0.90, 2, 60),
		withPerks(3, 400, 0.82, 3, 120),
	}
	require.NoError(t, validateVIPLadder(rising))

	// Equal rungs are allowed: a tier may improve only the discount.
	flat := []VIPTier{withPerks(1, 20, 0.95, 4, 60), withPerks(2, 100, 0.90, 4, 60)}
	require.NoError(t, validateVIPLadder(flat))

	shrinkingConcurrency := []VIPTier{withPerks(1, 20, 0.95, 8, 60), withPerks(2, 100, 0.90, 4, 60)}
	require.Error(t, validateVIPLadder(shrinkingConcurrency))

	shrinkingRPM := []VIPTier{withPerks(1, 20, 0.95, 4, 120), withPerks(2, 100, 0.90, 4, 60)}
	require.Error(t, validateVIPLadder(shrinkingRPM))
}

// An exemption outranks every finite amount, and it has to be compared that way
// rather than by the addend beside it. An unlimited top tier normally leaves
// rpm_limit at 0 because the number is unused, so comparing the raw values would
// read it as granting less than the tier below and reject a correct ladder.
func TestValidateVIPLadderRanksExemptionsAboveFiniteGrants(t *testing.T) {
	unlimitedTop := []VIPTier{
		ladderTier(1, 20, 0.95),
		func() VIPTier {
			tier := ladderTier(2, 100, 0.90)
			tier.Concurrency = 1
			tier.RPMLimit = 0
			tier.UnlimitedConcurrency = true
			tier.UnlimitedRPM = true
			return tier
		}(),
	}
	require.NoError(t, validateVIPLadder(unlimitedTop))

	// Inverted: the exemption sits on the cheaper rung, so the expensive tier is
	// strictly worse than the one below it.
	unlimitedBottom := []VIPTier{
		func() VIPTier {
			tier := ladderTier(1, 20, 0.95)
			tier.UnlimitedConcurrency = true
			return tier
		}(),
		ladderTier(2, 100, 0.90),
	}
	require.Error(t, validateVIPLadder(unlimitedBottom))
}

// rpm_limit 0 is a real configuration ("this tier adds no RPM"), unlike
// concurrency where 0 is both meaningless and unstorable.
func TestValidateVIPTierFieldsAllowsZeroRPMButRejectsNegative(t *testing.T) {
	base := ladderTier(1, 20, 0.95)

	noRPMGrant := base
	noRPMGrant.RPMLimit = 0
	require.NoError(t, validateVIPTierFields(noRPMGrant))

	negative := base
	negative.RPMLimit = -1
	require.Error(t, validateVIPTierFields(negative))
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
