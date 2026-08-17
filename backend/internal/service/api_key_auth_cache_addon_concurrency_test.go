package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type stubAddonResolver struct {
	holdings AddonHoldings
	err      error
	calls    int
}

func (s *stubAddonResolver) ResolveActiveAddons(context.Context, int64) (AddonHoldings, error) {
	s.calls++
	return s.holdings, s.err
}

func addonConcurrencyAPIKey(userConcurrency, userRPM int) *APIKey {
	return &APIKey{
		ID:     93,
		UserID: 44,
		Name:   "addon-concurrency",
		Status: StatusActive,
		User: &User{
			ID:          44,
			Email:       "addon-concurrency@test.local",
			Status:      StatusActive,
			Concurrency: userConcurrency,
			RPMLimit:    userRPM,
		},
	}
}

// Four things a user can hold, four addends. Folding any of them into a max()
// is how somebody ends up paying twice for one number: a VIP whose tier grants
// more than they bought would see the purchase do nothing at all.
func TestAPIKeyAuthSnapshotStacksAddonOnVIPAndResellerPlan(t *testing.T) {
	vip := &stubVIPBenefitRepo{concurrency: 5}
	plan := &stubAuthPlanResolver{assignment: activePlanAssignment(10)}
	addons := &stubAddonResolver{holdings: AddonHoldings{Concurrency: 4}}
	svc := &APIKeyService{vipBenefitRepo: vip, resellerPlanResolver: plan, addonResolver: addons}

	snapshot := svc.snapshotFromAPIKey(context.Background(), addonConcurrencyAPIKey(2, 0))

	require.NotNil(t, snapshot)
	require.Equal(t, 21, snapshot.User.Concurrency, "2 own + 5 VIP + 10 reseller + 4 purchased")
	require.Equal(t, 1, addons.calls, "add-ons are resolved once per snapshot, not per request")
}

func TestAPIKeyAuthSnapshotAppliesAddonConcurrencyAlone(t *testing.T) {
	addons := &stubAddonResolver{holdings: AddonHoldings{Concurrency: 6}}
	svc := &APIKeyService{addonResolver: addons}

	snapshot := svc.snapshotFromAPIKey(context.Background(), addonConcurrencyAPIKey(3, 0))

	require.Equal(t, 9, snapshot.User.Concurrency)
}

// `users.rpm_limit` of 0 means *unlimited*. Adding a purchased 60 to it would
// convert "no cap" into "60 a minute" and throttle the very user who paid to
// be throttled less.
func TestAPIKeyAuthSnapshotAddsPurchasedRPMOnlyOnTopOfAnExistingLimit(t *testing.T) {
	for _, tc := range []struct {
		name     string
		userRPM  int
		addonRPM int
		want     int
	}{
		{name: "limited user gets the purchase added", userRPM: 120, addonRPM: 60, want: 180},
		{name: "unlimited user stays unlimited", userRPM: 0, addonRPM: 60, want: 0},
		{name: "no purchase changes nothing", userRPM: 120, addonRPM: 0, want: 120},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &APIKeyService{addonResolver: &stubAddonResolver{holdings: AddonHoldings{RPM: tc.addonRPM}}}

			snapshot := svc.snapshotFromAPIKey(context.Background(), addonConcurrencyAPIKey(1, tc.userRPM))

			require.Equal(t, tc.want, snapshot.User.RPMLimit)
		})
	}
}

// Same failure posture as the VIP and reseller lookups: losing the perk is
// recoverable, refusing to build the snapshot is not.
func TestAPIKeyAuthSnapshotKeepsPlainLimitsWhenAddonLookupFails(t *testing.T) {
	addons := &stubAddonResolver{
		holdings: AddonHoldings{Concurrency: 9, RPM: 300},
		err:      errors.New("add-on lookup unavailable"),
	}
	svc := &APIKeyService{addonResolver: addons}

	snapshot := svc.snapshotFromAPIKey(context.Background(), addonConcurrencyAPIKey(2, 120))

	require.Equal(t, 2, snapshot.User.Concurrency)
	require.Equal(t, 120, snapshot.User.RPMLimit)
}

// No resolver wired is the behaviour before the store existed.
func TestAPIKeyAuthSnapshotWithoutAddonResolverKeepsPlainLimits(t *testing.T) {
	svc := &APIKeyService{}

	snapshot := svc.snapshotFromAPIKey(context.Background(), addonConcurrencyAPIKey(7, 90))

	require.Equal(t, 7, snapshot.User.Concurrency)
	require.Equal(t, 90, snapshot.User.RPMLimit)
}

// End to end through the real resolver: an expired row must stop counting on
// the snapshot immediately, without waiting for the sweep to delete it.
func TestAPIKeyAuthSnapshotIgnoresLapsedAddons(t *testing.T) {
	repo := newStubAddonRepo(0)
	repo.rows[AddonKindConcurrency] = &UserAddon{
		Kind: AddonKindConcurrency, Amount: 10, ExpiresAt: time.Now().Add(-time.Second),
	}
	svc := &APIKeyService{addonResolver: newTestAddonService(repo, nil)}

	snapshot := svc.snapshotFromAPIKey(context.Background(), addonConcurrencyAPIKey(2, 0))

	require.Equal(t, 2, snapshot.User.Concurrency)
}
