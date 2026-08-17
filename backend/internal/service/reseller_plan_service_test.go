package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type stubResellerPlanRepo struct {
	plans      map[int64]*ResellerPlan
	assignment *ResellerPlanAssignment

	updated     *ResellerPlan
	updateCalls int

	assignedUser   int64
	assignedPlan   *ResellerPlan
	assignedExpiry time.Time
	assignedCredit float64
	assignCalls    int
	cleared        bool
}

func (s *stubResellerPlanRepo) List(context.Context) ([]*ResellerPlan, error) { return nil, nil }

func (s *stubResellerPlanRepo) GetByID(_ context.Context, id int64) (*ResellerPlan, error) {
	return s.plans[id], nil
}

func (s *stubResellerPlanRepo) Update(_ context.Context, plan *ResellerPlan) (*ResellerPlan, error) {
	s.updateCalls++
	stored := *plan
	s.updated = &stored
	s.plans[plan.ID] = &stored
	return &stored, nil
}

func (s *stubResellerPlanRepo) AssignToUser(_ context.Context, userID int64, plan *ResellerPlan, expiresAt time.Time, credit float64) error {
	s.assignCalls++
	s.assignedUser = userID
	s.assignedPlan = plan
	s.assignedExpiry = expiresAt
	s.assignedCredit = credit
	return nil
}

func (s *stubResellerPlanRepo) GetUserAssignment(context.Context, int64) (*ResellerPlanAssignment, error) {
	return s.assignment, nil
}

func (s *stubResellerPlanRepo) ClearUserAssignment(context.Context, int64) error {
	s.cleared = true
	return nil
}

// Money: 0.7 × 400 in binary floating point is not 280, which is why this goes
// through decimal rather than a bare multiplication.
func TestCalculateResellerCredit(t *testing.T) {
	for _, tc := range []struct {
		name  string
		price float64
		rate  float64
		want  float64
	}{
		{name: "tier 1", price: 50, rate: 0.5, want: 25},
		{name: "tier 2", price: 150, rate: 0.6, want: 90},
		{name: "tier 3", price: 400, rate: 0.7, want: 280},
		{name: "rounds to cents", price: 99.99, rate: 0.65, want: 64.99},
		{name: "no credit rate", price: 100, rate: 0, want: 0},
		{name: "free plan", price: 0, rate: 0.7, want: 0},
		{name: "negative rate is not a refund", price: 100, rate: -0.5, want: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.InDelta(t, tc.want, calculateResellerCredit(tc.price, tc.rate), 1e-9)
		})
	}
}

func TestAssignPlanStampsTierAndCredits(t *testing.T) {
	plan := &ResellerPlan{ID: 3, Level: 3, Name: "Reseller 3", Price: 400, CreditRate: 0.7, ValidityDays: 365, MaxDomains: 10, Enabled: true}
	repo := &stubResellerPlanRepo{plans: map[int64]*ResellerPlan{3: plan}}
	svc := NewResellerPlanService(repo)

	before := time.Now()
	assignment, err := svc.AssignPlan(context.Background(), 42, 3)
	require.NoError(t, err)

	require.Equal(t, int64(42), repo.assignedUser)
	require.Equal(t, plan, repo.assignedPlan)
	require.InDelta(t, 280, repo.assignedCredit, 1e-9)
	require.True(t, repo.assignedExpiry.After(before.AddDate(0, 0, 364)))
	require.Equal(t, plan, assignment.Plan)
}

// Expiry runs from now, not from any previous plan's end date: stacking would
// let a reseller buy the cheapest tier over and over and accumulate years,
// while the credit pays out on every assignment regardless.
func TestAssignPlanDoesNotStackExpiryOnRepeatPurchase(t *testing.T) {
	plan := &ResellerPlan{ID: 1, Level: 1, Price: 50, CreditRate: 0.5, ValidityDays: 30, Enabled: true}
	repo := &stubResellerPlanRepo{
		plans:      map[int64]*ResellerPlan{1: plan},
		assignment: &ResellerPlanAssignment{Plan: plan, ExpiresAt: time.Now().AddDate(1, 0, 0)},
	}
	svc := NewResellerPlanService(repo)

	_, err := svc.AssignPlan(context.Background(), 42, 1)
	require.NoError(t, err)

	require.True(t, repo.assignedExpiry.Before(time.Now().AddDate(0, 0, 31)),
		"expiry must run from now, not extend the existing one")
}

func TestAssignPlanRejectsUnknownAndDisabledPlans(t *testing.T) {
	disabled := &ResellerPlan{ID: 9, Level: 9, Price: 10, ValidityDays: 30, Enabled: false}
	repo := &stubResellerPlanRepo{plans: map[int64]*ResellerPlan{9: disabled}}
	svc := NewResellerPlanService(repo)

	_, err := svc.AssignPlan(context.Background(), 42, 404)
	require.Error(t, err)

	_, err = svc.AssignPlan(context.Background(), 42, 9)
	require.Error(t, err)

	_, err = svc.AssignPlan(context.Background(), 0, 9)
	require.Error(t, err)

	require.Equal(t, 0, repo.assignCalls, "nothing may be written for a rejected assignment")
}

// The bounds are not decoration. A credit rate above 1 hands a reseller more
// balance than they paid, which mints money on every purchase; a validity of
// zero produces a plan that is already expired the instant it is assigned, so
// the buyer pays and gets nothing. Both have to be refused before the write.
func TestUpdateRejectsOutOfBoundsFields(t *testing.T) {
	stored := ResellerPlan{
		ID: 2, Level: 2, Name: "Reseller 2",
		Price: 150, CreditRate: 0.6, ConcurrencyBonus: 5,
		RPMLimit: 120, MaxDomains: 3, ValidityDays: 365, Enabled: true,
	}

	for _, tc := range []struct {
		name   string
		in     ResellerPlanUpdate
		wantOK bool
	}{
		{name: "no-op", in: ResellerPlanUpdate{}, wantOK: true},
		{name: "zero price is free, not invalid", in: ResellerPlanUpdate{Price: float64Ptr(0)}, wantOK: true},
		{name: "negative price", in: ResellerPlanUpdate{Price: float64Ptr(-0.01)}},
		{name: "credit rate zero", in: ResellerPlanUpdate{CreditRate: float64Ptr(0)}, wantOK: true},
		{name: "credit rate one", in: ResellerPlanUpdate{CreditRate: float64Ptr(1)}, wantOK: true},
		{name: "credit rate above one mints money", in: ResellerPlanUpdate{CreditRate: float64Ptr(1.01)}},
		{name: "negative credit rate", in: ResellerPlanUpdate{CreditRate: float64Ptr(-0.01)}},
		{name: "validity one day", in: ResellerPlanUpdate{ValidityDays: intPtr(1)}, wantOK: true},
		{name: "validity zero is born expired", in: ResellerPlanUpdate{ValidityDays: intPtr(0)}},
		{name: "negative validity", in: ResellerPlanUpdate{ValidityDays: intPtr(-1)}},
		{name: "zero concurrency bonus", in: ResellerPlanUpdate{ConcurrencyBonus: intPtr(0)}, wantOK: true},
		{name: "negative concurrency bonus", in: ResellerPlanUpdate{ConcurrencyBonus: intPtr(-1)}},
		{name: "zero rpm means no override", in: ResellerPlanUpdate{RPMLimit: intPtr(0)}, wantOK: true},
		{name: "negative rpm", in: ResellerPlanUpdate{RPMLimit: intPtr(-1)}},
		{name: "zero max domains", in: ResellerPlanUpdate{MaxDomains: intPtr(0)}, wantOK: true},
		{name: "negative max domains", in: ResellerPlanUpdate{MaxDomains: intPtr(-1)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := stored
			repo := &stubResellerPlanRepo{plans: map[int64]*ResellerPlan{2: &plan}}
			svc := NewResellerPlanService(repo)

			got, err := svc.Update(context.Background(), 2, tc.in)
			if !tc.wantOK {
				require.Error(t, err)
				require.Nil(t, got)
				require.Zero(t, repo.updateCalls, "a rejected edit must never reach the database")
				return
			}

			require.NoError(t, err)
			require.Equal(t, 1, repo.updateCalls)
			require.NotNil(t, got)
		})
	}
}

// Nil fields mean "leave alone": an operator toggling a tier off must not have
// to resend the price, and resending nothing must not zero the row.
func TestUpdateLeavesOmittedFieldsAlone(t *testing.T) {
	plan := ResellerPlan{
		ID: 2, Level: 2, Name: "Reseller 2",
		Price: 150, CreditRate: 0.6, ConcurrencyBonus: 5,
		RPMLimit: 120, MaxDomains: 3, ValidityDays: 365,
		AllowedGroupIDs: []int64{7}, Enabled: true,
	}
	repo := &stubResellerPlanRepo{plans: map[int64]*ResellerPlan{2: &plan}}
	svc := NewResellerPlanService(repo)

	off := false
	got, err := svc.Update(context.Background(), 2, ResellerPlanUpdate{Enabled: &off})
	require.NoError(t, err)

	require.False(t, got.Enabled)
	require.InDelta(t, 150, got.Price, 1e-9)
	require.InDelta(t, 0.6, got.CreditRate, 1e-9)
	require.Equal(t, 5, got.ConcurrencyBonus)
	require.Equal(t, 120, got.RPMLimit)
	require.Equal(t, 3, got.MaxDomains)
	require.Equal(t, 365, got.ValidityDays)
	require.Equal(t, []int64{7}, got.AllowedGroupIDs)
	// Level and name are not editable at all — the ladder orders by level, and a
	// rename changes what resellers believe they bought.
	require.Equal(t, 2, got.Level)
	require.Equal(t, "Reseller 2", got.Name)
}

// The whitelist arrives as a slice off a decoded request body. Storing the
// caller's backing array would let a later mutation of it change what we think
// we wrote.
func TestUpdateCopiesAllowedGroupIDs(t *testing.T) {
	plan := ResellerPlan{ID: 2, Level: 2, Price: 150, CreditRate: 0.6, ValidityDays: 365, Enabled: true}
	repo := &stubResellerPlanRepo{plans: map[int64]*ResellerPlan{2: &plan}}
	svc := NewResellerPlanService(repo)

	ids := []int64{1, 2}
	got, err := svc.Update(context.Background(), 2, ResellerPlanUpdate{AllowedGroupIDs: &ids})
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2}, got.AllowedGroupIDs)

	ids[0] = 999
	require.Equal(t, []int64{1, 2}, repo.updated.AllowedGroupIDs)
}

func TestUpdateRejectsUnknownPlan(t *testing.T) {
	repo := &stubResellerPlanRepo{plans: map[int64]*ResellerPlan{}}
	svc := NewResellerPlanService(repo)

	_, err := svc.Update(context.Background(), 404, ResellerPlanUpdate{Price: float64Ptr(10)})
	require.Error(t, err)

	_, err = svc.Update(context.Background(), 0, ResellerPlanUpdate{Price: float64Ptr(10)})
	require.Error(t, err)

	require.Zero(t, repo.updateCalls)
}

func TestMaxDomainsForUser(t *testing.T) {
	plan := &ResellerPlan{ID: 2, Level: 2, MaxDomains: 3, ValidityDays: 365, Enabled: true}

	t.Run("active plan grants its quota", func(t *testing.T) {
		repo := &stubResellerPlanRepo{assignment: &ResellerPlanAssignment{Plan: plan, ExpiresAt: time.Now().Add(time.Hour)}}
		got, err := NewResellerPlanService(repo).MaxDomainsForUser(context.Background(), 42)
		require.NoError(t, err)
		require.Equal(t, 3, got)
	})

	// An expired plan must stop granting domains, otherwise a one-month tier
	// keeps its certificates alive forever.
	t.Run("expired plan grants nothing", func(t *testing.T) {
		repo := &stubResellerPlanRepo{assignment: &ResellerPlanAssignment{Plan: plan, ExpiresAt: time.Now().Add(-time.Hour)}}
		got, err := NewResellerPlanService(repo).MaxDomainsForUser(context.Background(), 42)
		require.NoError(t, err)
		require.Equal(t, 0, got)
	})

	t.Run("disabled plan grants nothing", func(t *testing.T) {
		off := *plan
		off.Enabled = false
		repo := &stubResellerPlanRepo{assignment: &ResellerPlanAssignment{Plan: &off, ExpiresAt: time.Now().Add(time.Hour)}}
		got, err := NewResellerPlanService(repo).MaxDomainsForUser(context.Background(), 42)
		require.NoError(t, err)
		require.Equal(t, 0, got)
	})

	t.Run("no plan grants nothing", func(t *testing.T) {
		got, err := NewResellerPlanService(&stubResellerPlanRepo{}).MaxDomainsForUser(context.Background(), 42)
		require.NoError(t, err)
		require.Equal(t, 0, got)
	})
}
