package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type stubPlanResolver struct{ assignment *ResellerPlanAssignment }

func (s stubPlanResolver) ResolveForUser(context.Context, int64) (*ResellerPlanAssignment, error) {
	return s.assignment, nil
}

type stubCodeRepo struct {
	calls     int
	lastTotal float64
	lastCodes []RedeemCode
	err       error
}

func (s *stubCodeRepo) GenerateForReseller(_ context.Context, _ int64, total float64, codes []RedeemCode) error {
	s.calls++
	s.lastTotal = total
	s.lastCodes = codes
	return s.err
}

func activeReseller(plan *ResellerPlan) stubPlanResolver {
	return stubPlanResolver{assignment: &ResellerPlanAssignment{Plan: plan, ExpiresAt: time.Now().Add(time.Hour)}}
}

func TestGenerateResellerCodesDebitsFaceValueAndStampsOwner(t *testing.T) {
	plan := &ResellerPlan{ID: 2, Level: 2, MaxDomains: 3, Enabled: true}
	repo := &stubCodeRepo{}
	svc := &RedeemService{}

	codes, err := svc.GenerateResellerCodes(context.Background(), activeReseller(plan), repo,
		ResellerCodeRequest{UserID: 42, Count: 3, Value: 10})
	require.NoError(t, err)
	require.Len(t, codes, 3)

	// Face value, no discount: the tier's discount was already paid out once as
	// the credit that came with the plan.
	require.InDelta(t, 30, repo.lastTotal, 1e-9)

	for _, code := range repo.lastCodes {
		require.NotNil(t, code.CreatedBy)
		require.Equal(t, int64(42), *code.CreatedBy)
		require.Equal(t, RedeemTypeBalance, code.Type)
		require.Equal(t, StatusUnused, code.Status)
		require.NotEmpty(t, code.Code)
	}
}

// count × value is money: 3 × 0.1 in binary floating point is not 0.3.
func TestGenerateResellerCodesTotalUsesDecimal(t *testing.T) {
	repo := &stubCodeRepo{}
	svc := &RedeemService{}

	_, err := svc.GenerateResellerCodes(context.Background(),
		activeReseller(&ResellerPlan{ID: 1, Enabled: true}), repo,
		ResellerCodeRequest{UserID: 42, Count: 3, Value: 0.1})
	require.NoError(t, err)
	require.Equal(t, 0.3, repo.lastTotal)
}

func TestGenerateResellerCodesRequiresAnActivePlan(t *testing.T) {
	repo := &stubCodeRepo{}
	svc := &RedeemService{}
	plan := &ResellerPlan{ID: 1, Enabled: true}

	t.Run("no plan", func(t *testing.T) {
		_, err := svc.GenerateResellerCodes(context.Background(), stubPlanResolver{}, repo,
			ResellerCodeRequest{UserID: 42, Count: 1, Value: 10})
		require.Error(t, err)
	})

	t.Run("expired plan", func(t *testing.T) {
		expired := stubPlanResolver{assignment: &ResellerPlanAssignment{Plan: plan, ExpiresAt: time.Now().Add(-time.Hour)}}
		_, err := svc.GenerateResellerCodes(context.Background(), expired, repo,
			ResellerCodeRequest{UserID: 42, Count: 1, Value: 10})
		require.Error(t, err)
	})

	require.Equal(t, 0, repo.calls, "nothing may be debited for a rejected request")
}

func TestGenerateResellerCodesRejectsBadBatches(t *testing.T) {
	repo := &stubCodeRepo{}
	svc := &RedeemService{}
	resolver := activeReseller(&ResellerPlan{ID: 1, Enabled: true})

	for _, tc := range []struct {
		name string
		req  ResellerCodeRequest
	}{
		{name: "zero count", req: ResellerCodeRequest{UserID: 42, Count: 0, Value: 10}},
		{name: "negative count", req: ResellerCodeRequest{UserID: 42, Count: -1, Value: 10}},
		// Bounded below the admin limit of 1000: this path spends real balance,
		// and a stolen session should not mint a thousand bearer instruments.
		{name: "over batch cap", req: ResellerCodeRequest{UserID: 42, Count: maxResellerCodesPerBatch + 1, Value: 10}},
		{name: "zero value", req: ResellerCodeRequest{UserID: 42, Count: 1, Value: 0}},
		{name: "negative value", req: ResellerCodeRequest{UserID: 42, Count: 1, Value: -5}},
		{name: "no user", req: ResellerCodeRequest{UserID: 0, Count: 1, Value: 10}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.GenerateResellerCodes(context.Background(), resolver, repo, tc.req)
			require.Error(t, err)
		})
	}

	require.Equal(t, 0, repo.calls)
}

// Tiers sell the groups they were sold. Resellers pick from groups we define
// rather than setting rates, so the whitelist is the only product lever they
// have — and it has to actually bind.
func TestGenerateResellerCodesEnforcesGroupWhitelist(t *testing.T) {
	allowed := int64(7)
	denied := int64(9)
	plan := &ResellerPlan{ID: 1, Enabled: true, AllowedGroupIDs: []int64{allowed}}
	repo := &stubCodeRepo{}
	svc := &RedeemService{}

	_, err := svc.GenerateResellerCodes(context.Background(), activeReseller(plan), repo,
		ResellerCodeRequest{UserID: 42, Count: 1, Value: 10, GroupID: &allowed})
	require.NoError(t, err)

	_, err = svc.GenerateResellerCodes(context.Background(), activeReseller(plan), repo,
		ResellerCodeRequest{UserID: 42, Count: 1, Value: 10, GroupID: &denied})
	require.Error(t, err)

	require.Equal(t, 1, repo.calls, "only the allowed group may reach the repository")
}

// An empty whitelist means unrestricted, which is what a tier with no group
// limits configured has to mean — otherwise every plan blocks everything.
func TestGenerateResellerCodesEmptyWhitelistAllowsAnyGroup(t *testing.T) {
	group := int64(9)
	repo := &stubCodeRepo{}
	svc := &RedeemService{}

	_, err := svc.GenerateResellerCodes(context.Background(),
		activeReseller(&ResellerPlan{ID: 1, Enabled: true}), repo,
		ResellerCodeRequest{UserID: 42, Count: 1, Value: 10, GroupID: &group})
	require.NoError(t, err)
	require.Equal(t, 1, repo.calls)
}
