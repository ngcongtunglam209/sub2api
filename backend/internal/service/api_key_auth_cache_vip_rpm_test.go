package service

// VIP 等级除并发外还赠送 RPM，最高档可免除上限。
//
// 这里钉两件事：加数的叠加语义，以及免除上限**最后**生效的顺序。后者是本次
// 改动最容易写错的地方——若在 VIP 那一段就把上限清成 0，下面的分销商套餐与
// 加购项会把各自的加数加到 0 上，"无上限"退化成一个很小的有限值，而且从等级
// 配置上完全看不出来。

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func vipRPMTestAPIKey(userConcurrency, userRPM int) *APIKey {
	return &APIKey{
		ID:     94,
		UserID: 45,
		Name:   "vip-rpm",
		Status: StatusActive,
		User: &User{
			ID:          45,
			Email:       "vip-rpm@test.local",
			Status:      StatusActive,
			Concurrency: userConcurrency,
			RPMLimit:    userRPM,
		},
	}
}

// The same asymmetry the add-on path already documents: `users.rpm_limit` of 0
// means unlimited, so adding a tier's 60 to it would cap a user who had no cap.
func TestAPIKeyAuthSnapshotAddsVIPRPMOnlyOnTopOfAnExistingLimit(t *testing.T) {
	for _, tc := range []struct {
		name    string
		userRPM int
		tierRPM int
		want    int
	}{
		{name: "limited user gets the bonus added", userRPM: 120, tierRPM: 60, want: 180},
		{name: "unlimited user stays unlimited", userRPM: 0, tierRPM: 60, want: 0},
		{name: "tier granting no rpm changes nothing", userRPM: 120, tierRPM: 0, want: 120},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &APIKeyService{vipBenefitRepo: &stubVIPBenefitRepo{rpm: tc.tierRPM}}

			snapshot := svc.snapshotFromAPIKey(context.Background(), vipRPMTestAPIKey(1, tc.userRPM))

			require.Equal(t, tc.want, snapshot.User.RPMLimit)
		})
	}
}

// The ordering guarantee, stated as the bug it prevents.
//
// VIP resolves before the reseller plan and the add-ons. If the exemption were
// applied where it is resolved, those two would then add 10 and 4 to a cleared
// 0 and produce a ceiling of 14 — lower than the 16 the same user would have
// had with no exemption at all, and lower than every tier below it.
func TestAPIKeyAuthSnapshotUnlimitedVIPConcurrencySurvivesLaterAddends(t *testing.T) {
	svc := &APIKeyService{
		vipBenefitRepo:       &stubVIPBenefitRepo{concurrency: 5, unlimitedConcurrency: true},
		resellerPlanResolver: &stubAuthPlanResolver{assignment: activePlanAssignment(10)},
		addonResolver:        &stubAddonResolver{holdings: AddonHoldings{Concurrency: 4}},
	}

	snapshot := svc.snapshotFromAPIKey(context.Background(), vipRPMTestAPIKey(2, 0))

	require.Zero(t, snapshot.User.Concurrency, "0 is what ConcurrencyService reads as no ceiling")
}

func TestAPIKeyAuthSnapshotUnlimitedVIPRPMSurvivesLaterAddends(t *testing.T) {
	svc := &APIKeyService{
		vipBenefitRepo: &stubVIPBenefitRepo{rpm: 30, unlimitedRPM: true},
		addonResolver:  &stubAddonResolver{holdings: AddonHoldings{RPM: 60}},
	}

	snapshot := svc.snapshotFromAPIKey(context.Background(), vipRPMTestAPIKey(1, 120))

	require.Zero(t, snapshot.User.RPMLimit, "0 is what the RPM check reads as no ceiling")
}

// The two exemptions are independent: a tier may lift the RPM ceiling while
// still rationing slots, which is the sane shape given slots are the scarce
// resource and requests per minute are not.
func TestAPIKeyAuthSnapshotAppliesVIPExemptionsIndependently(t *testing.T) {
	svc := &APIKeyService{
		vipBenefitRepo: &stubVIPBenefitRepo{concurrency: 4, unlimitedRPM: true},
	}

	snapshot := svc.snapshotFromAPIKey(context.Background(), vipRPMTestAPIKey(2, 120))

	require.Equal(t, 6, snapshot.User.Concurrency, "2 own + 4 tier, still bounded")
	require.Zero(t, snapshot.User.RPMLimit)
}

// A failed lookup must not lift a ceiling. Losing a perk is recoverable; handing
// out an exemption nobody paid for silently uncaps the fleet's scarcest
// resource, and does so only while the database is already unwell.
func TestAPIKeyAuthSnapshotDoesNotExemptWhenVIPLookupFails(t *testing.T) {
	svc := &APIKeyService{
		vipBenefitRepo: &stubVIPBenefitRepo{
			unlimitedConcurrency: true,
			unlimitedRPM:         true,
			err:                  errors.New("vip lookup unavailable"),
		},
	}

	snapshot := svc.snapshotFromAPIKey(context.Background(), vipRPMTestAPIKey(3, 120))

	require.Equal(t, 3, snapshot.User.Concurrency)
	require.Equal(t, 120, snapshot.User.RPMLimit)
}
