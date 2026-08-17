package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type stubAuthPlanResolver struct {
	assignment *ResellerPlanAssignment
	err        error
	calls      int
}

func (s *stubAuthPlanResolver) ResolveForUser(context.Context, int64) (*ResellerPlanAssignment, error) {
	s.calls++
	return s.assignment, s.err
}

func resellerConcurrencyAPIKey(userConcurrency int) *APIKey {
	return &APIKey{
		ID:     92,
		UserID: 43,
		Name:   "reseller-concurrency",
		Status: StatusActive,
		User: &User{
			ID:          43,
			Email:       "reseller-concurrency@test.local",
			Status:      StatusActive,
			Concurrency: userConcurrency,
		},
	}
}

func activePlanAssignment(bonus int) *ResellerPlanAssignment {
	return &ResellerPlanAssignment{
		Plan:      &ResellerPlan{ID: 3, Level: 3, ConcurrencyBonus: bonus, Enabled: true},
		ExpiresAt: time.Now().Add(time.Hour),
	}
}

// The reseller bonus stacks on the VIP bonus rather than replacing it: the two
// were bought separately, and folding one into the other is how a customer ends
// up paying twice for one number.
func TestAPIKeyAuthSnapshotStacksResellerBonusOnTopOfVIP(t *testing.T) {
	vip := &stubVIPBenefitRepo{concurrency: 5}
	plan := &stubAuthPlanResolver{assignment: activePlanAssignment(10)}
	svc := &APIKeyService{vipBenefitRepo: vip, resellerPlanResolver: plan}

	snapshot := svc.snapshotFromAPIKey(context.Background(), resellerConcurrencyAPIKey(2))

	require.NotNil(t, snapshot)
	require.Equal(t, 17, snapshot.User.Concurrency, "2 own + 5 VIP + 10 reseller")
	require.Equal(t, 1, plan.calls, "the plan is resolved once per snapshot, not per request")
}

func TestAPIKeyAuthSnapshotAppliesResellerBonusWithoutVIP(t *testing.T) {
	plan := &stubAuthPlanResolver{assignment: activePlanAssignment(10)}
	svc := &APIKeyService{resellerPlanResolver: plan}

	snapshot := svc.snapshotFromAPIKey(context.Background(), resellerConcurrencyAPIKey(2))

	require.Equal(t, 12, snapshot.User.Concurrency)
}

// A lapsed or switched-off tier must stop conferring immediately, not at the
// next refresh of some other subsystem.
func TestAPIKeyAuthSnapshotIgnoresInactiveResellerPlans(t *testing.T) {
	expired := &ResellerPlanAssignment{
		Plan:      &ResellerPlan{ID: 3, ConcurrencyBonus: 10, Enabled: true},
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	disabled := &ResellerPlanAssignment{
		Plan:      &ResellerPlan{ID: 3, ConcurrencyBonus: 10, Enabled: false},
		ExpiresAt: time.Now().Add(time.Hour),
	}

	for name, assignment := range map[string]*ResellerPlanAssignment{
		"expired":  expired,
		"disabled": disabled,
		"none":     nil,
	} {
		t.Run(name, func(t *testing.T) {
			svc := &APIKeyService{resellerPlanResolver: &stubAuthPlanResolver{assignment: assignment}}
			snapshot := svc.snapshotFromAPIKey(context.Background(), resellerConcurrencyAPIKey(2))
			require.Equal(t, 2, snapshot.User.Concurrency)
		})
	}
}

// Same posture as the VIP lookup: losing the perk is recoverable, refusing to
// build the snapshot is not.
func TestAPIKeyAuthSnapshotKeepsUserConcurrencyWhenPlanLookupFails(t *testing.T) {
	plan := &stubAuthPlanResolver{assignment: activePlanAssignment(10), err: errors.New("plan lookup unavailable")}
	svc := &APIKeyService{resellerPlanResolver: plan}

	snapshot := svc.snapshotFromAPIKey(context.Background(), resellerConcurrencyAPIKey(2))

	require.Equal(t, 2, snapshot.User.Concurrency)
}

// No resolver wired is the behaviour before reseller plans existed.
func TestAPIKeyAuthSnapshotWithoutPlanResolverKeepsUserConcurrency(t *testing.T) {
	svc := &APIKeyService{}

	snapshot := svc.snapshotFromAPIKey(context.Background(), resellerConcurrencyAPIKey(7))

	require.Equal(t, 7, snapshot.User.Concurrency)
}
