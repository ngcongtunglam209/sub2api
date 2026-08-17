package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// stubAddonRepo is an in-memory stand-in for the store's persistence.
//
// RunInTx really does roll back: it snapshots the balance and the rows and
// restores them when fn returns an error. Without that the atomicity tests
// below would pass against an implementation that debits and then writes
// nothing, which is the exact bug they exist to catch.
type stubAddonRepo struct {
	balance float64
	rows    map[AddonKind]*UserAddon

	debits     []float64
	listErr    error
	upsertErr  error
	txDepth    int
	lockCalls  int
	nextRowID  int64
	rollbacks  int
	debitCalls int
}

func newStubAddonRepo(balance float64) *stubAddonRepo {
	return &stubAddonRepo{balance: balance, rows: map[AddonKind]*UserAddon{}}
}

func (s *stubAddonRepo) ListByUser(context.Context, int64) ([]*UserAddon, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	out := make([]*UserAddon, 0, len(s.rows))
	for _, row := range s.rows {
		copied := *row
		out = append(out, &copied)
	}
	return out, nil
}

func (s *stubAddonRepo) LockByUserKind(_ context.Context, _ int64, kind AddonKind) (*UserAddon, error) {
	s.lockCalls++
	row, ok := s.rows[kind]
	if !ok {
		return nil, nil
	}
	copied := *row
	return &copied, nil
}

func (s *stubAddonRepo) Upsert(_ context.Context, userID int64, kind AddonKind, amount int, expiresAt time.Time) (*UserAddon, error) {
	if s.upsertErr != nil {
		return nil, s.upsertErr
	}
	row, ok := s.rows[kind]
	if !ok {
		s.nextRowID++
		row = &UserAddon{ID: s.nextRowID, UserID: userID, Kind: kind}
		s.rows[kind] = row
	}
	row.Amount = amount
	row.ExpiresAt = expiresAt
	copied := *row
	return &copied, nil
}

func (s *stubAddonRepo) DebitBalanceGuarded(_ context.Context, _ int64, amount float64) error {
	s.debitCalls++
	// The real repository puts this comparison inside the UPDATE's WHERE
	// clause; the stub only has to reproduce the refusal.
	if s.balance < amount {
		return infraerrors.BadRequest("INSUFFICIENT_BALANCE", "not enough balance for this purchase")
	}
	s.debits = append(s.debits, amount)
	s.balance -= amount
	return nil
}

func (s *stubAddonRepo) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	s.txDepth++
	balanceBefore := s.balance
	debitsBefore := append([]float64(nil), s.debits...)
	rowsBefore := make(map[AddonKind]*UserAddon, len(s.rows))
	for kind, row := range s.rows {
		copied := *row
		rowsBefore[kind] = &copied
	}

	if err := fn(ctx); err != nil {
		s.rollbacks++
		s.balance = balanceBefore
		s.debits = debitsBefore
		s.rows = rowsBefore
		return err
	}
	return nil
}

func (s *stubAddonRepo) ListExpiredAddonUserIDs(context.Context, time.Time, int) ([]int64, error) {
	return nil, nil
}

func (s *stubAddonRepo) DeleteExpiredAddons(context.Context, time.Time, []int64) (int, error) {
	return 0, nil
}

var _ UserAddonRepository = (*stubAddonRepo)(nil)

// stubAddonSettingRepo backs the pricing service with a plain map.
type stubAddonSettingRepo struct {
	values map[string]string
	getErr error
	setErr error
}

func newStubAddonSettingRepo() *stubAddonSettingRepo {
	return &stubAddonSettingRepo{values: map[string]string{}}
}

func (s *stubAddonSettingRepo) Get(context.Context, string) (*Setting, error) { return nil, nil }

func (s *stubAddonSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	return s.values[key], nil
}

func (s *stubAddonSettingRepo) Set(_ context.Context, key, value string) error {
	s.values[key] = value
	return nil
}

func (s *stubAddonSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if v, ok := s.values[key]; ok {
			out[key] = v
		}
	}
	return out, nil
}

func (s *stubAddonSettingRepo) SetMultiple(_ context.Context, settings map[string]string) error {
	if s.setErr != nil {
		return s.setErr
	}
	for key, value := range settings {
		s.values[key] = value
	}
	return nil
}

func (s *stubAddonSettingRepo) GetAll(context.Context) (map[string]string, error) {
	return s.values, nil
}

func (s *stubAddonSettingRepo) Delete(_ context.Context, key string) error {
	delete(s.values, key)
	return nil
}

var _ SettingRepository = (*stubAddonSettingRepo)(nil)

// stubAddonPlanRepo is a reseller plan repository with error injection, which
// the shared stubResellerPlanRepo does not offer.
type stubAddonPlanRepo struct {
	plans      map[int64]*ResellerPlan
	assignment *ResellerPlanAssignment

	assignErr      error
	assignCalls    int
	assignedCredit float64
	assignedExpiry time.Time
}

func (s *stubAddonPlanRepo) List(context.Context) ([]*ResellerPlan, error) { return nil, nil }

func (s *stubAddonPlanRepo) GetByID(_ context.Context, id int64) (*ResellerPlan, error) {
	return s.plans[id], nil
}

func (s *stubAddonPlanRepo) Update(_ context.Context, plan *ResellerPlan) (*ResellerPlan, error) {
	return plan, nil
}

func (s *stubAddonPlanRepo) AssignToUser(_ context.Context, _ int64, _ *ResellerPlan, expiresAt time.Time, credit float64) error {
	s.assignCalls++
	if s.assignErr != nil {
		return s.assignErr
	}
	s.assignedCredit = credit
	s.assignedExpiry = expiresAt
	return nil
}

func (s *stubAddonPlanRepo) GetUserAssignment(context.Context, int64) (*ResellerPlanAssignment, error) {
	return s.assignment, nil
}

func (s *stubAddonPlanRepo) ClearUserAssignment(context.Context, int64) error { return nil }

var _ ResellerPlanRepository = (*stubAddonPlanRepo)(nil)

func newTestAddonService(repo *stubAddonRepo, planRepo *stubAddonPlanRepo) *AddonService {
	settings := newStubAddonSettingRepo()
	var planService *ResellerPlanService
	if planRepo != nil {
		planService = NewResellerPlanService(planRepo)
	}
	return NewAddonService(repo, NewAddonPricingService(settings), planService)
}

// Money: amount × unit price × months goes through decimal, because 3 × 0.1 in
// binary floating point is not 0.3 and a store that is a cent out on every
// order is a store somebody eventually reconciles by hand.
func TestCalculateAddonPrice(t *testing.T) {
	for _, tc := range []struct {
		name      string
		units     int
		unitPrice float64
		months    int
		want      float64
	}{
		{name: "one slot one month", units: 1, unitPrice: 2, months: 1, want: 2},
		{name: "five slots twelve months", units: 5, unitPrice: 2, months: 12, want: 120},
		{name: "rpm two blocks six months", units: 2, unitPrice: 1, months: 6, want: 12},
		{name: "float unit price that binary cannot hold", units: 3, unitPrice: 0.1, months: 1, want: 0.3},
		{name: "rounds to cents", units: 7, unitPrice: 0.335, months: 1, want: 2.35},
		{name: "zero units is free", units: 0, unitPrice: 2, months: 3, want: 0},
		{name: "zero months is free", units: 4, unitPrice: 2, months: 0, want: 0},
		{name: "negative units are not a refund", units: -5, unitPrice: 2, months: 1, want: 0},
		{name: "negative price is not a refund", units: 5, unitPrice: -2, months: 1, want: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.InDelta(t, tc.want, calculateAddonPrice(tc.units, tc.unitPrice, tc.months), 1e-9)
		})
	}
}

// RPM is sold in blocks. A partial block would have to round somewhere, and
// every rounding of a price is a place a customer and an invoice disagree.
func TestAddonPricedUnits(t *testing.T) {
	for _, tc := range []struct {
		name   string
		kind   AddonKind
		amount int
		want   int
		wantOK bool
	}{
		{name: "one slot", kind: AddonKindConcurrency, amount: 1, want: 1, wantOK: true},
		{name: "twenty slots", kind: AddonKindConcurrency, amount: 20, want: 20, wantOK: true},
		{name: "zero slots", kind: AddonKindConcurrency, amount: 0},
		{name: "negative slots", kind: AddonKindConcurrency, amount: -1},
		{name: "one rpm block", kind: AddonKindRPM, amount: 30, want: 1, wantOK: true},
		{name: "three rpm blocks", kind: AddonKindRPM, amount: 90, want: 3, wantOK: true},
		{name: "partial rpm block", kind: AddonKindRPM, amount: 45},
		{name: "below one rpm block", kind: AddonKindRPM, amount: 29},
		{name: "unknown kind", kind: AddonKind("gpus"), amount: 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := addonPricedUnits(tc.kind, tc.amount)
			if !tc.wantOK {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// months and amount are bounded before anything is written: a rejected order
// must never reach the database, let alone the balance.
func TestPurchaseRejectsOutOfBoundsOrders(t *testing.T) {
	for _, tc := range []struct {
		name   string
		kind   AddonKind
		amount int
		months int
	}{
		{name: "zero months", kind: AddonKindConcurrency, amount: 1, months: 0},
		{name: "negative months", kind: AddonKindConcurrency, amount: 1, months: -3},
		{name: "thirteen months", kind: AddonKindConcurrency, amount: 1, months: 13},
		{name: "a hundred years", kind: AddonKindConcurrency, amount: 1, months: 1200},
		{name: "zero amount", kind: AddonKindConcurrency, amount: 0, months: 1},
		{name: "negative amount", kind: AddonKindConcurrency, amount: -5, months: 1},
		{name: "partial rpm block", kind: AddonKindRPM, amount: 45, months: 1},
		{name: "unknown kind", kind: AddonKind("gpus"), amount: 1, months: 1},
		{name: "amount past the cap on its own", kind: AddonKindConcurrency, amount: 21, months: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newStubAddonRepo(1_000_000)
			svc := newTestAddonService(repo, nil)

			got, err := svc.Purchase(context.Background(), 42, tc.kind, tc.amount, tc.months)

			require.Error(t, err)
			require.Nil(t, got)
			require.InDelta(t, 1_000_000, repo.balance, 1e-9, "a rejected order must not touch the balance")
			require.Zero(t, repo.debitCalls)
			require.Empty(t, repo.rows, "a rejected order must not be written")
		})
	}
}

// The happy path: priced with the catalogue's defaults, debited once, stored
// once, and dated from now.
func TestPurchaseDebitsAndStores(t *testing.T) {
	repo := newStubAddonRepo(100)
	svc := newTestAddonService(repo, nil)

	before := time.Now()
	got, err := svc.Purchase(context.Background(), 42, AddonKindConcurrency, 3, 2)
	require.NoError(t, err)

	// 3 slots × $2 × 2 months.
	require.InDelta(t, 12, got.Price, 1e-9)
	require.InDelta(t, 88, repo.balance, 1e-9)
	require.Equal(t, []float64{12}, repo.debits)
	require.Equal(t, 3, got.HeldAfter)

	stored := repo.rows[AddonKindConcurrency]
	require.NotNil(t, stored)
	require.Equal(t, 3, stored.Amount)
	require.True(t, stored.ExpiresAt.After(before.AddDate(0, 2, 0).Add(-time.Minute)))
	require.True(t, stored.ExpiresAt.Before(before.AddDate(0, 2, 0).Add(time.Minute)))
}

// RPM is priced per block but stored in RPM: 90 RPM is three blocks at $1, and
// what lands in the row is 90, not 3.
func TestPurchaseRPMPricesPerBlockAndStoresRPM(t *testing.T) {
	repo := newStubAddonRepo(100)
	svc := newTestAddonService(repo, nil)

	got, err := svc.Purchase(context.Background(), 42, AddonKindRPM, 90, 3)
	require.NoError(t, err)

	require.InDelta(t, 9, got.Price, 1e-9) // 3 blocks × $1 × 3 months
	require.Equal(t, 90, repo.rows[AddonKindRPM].Amount)
	require.Equal(t, 90, got.HeldAfter)
}

// The cap is checked against what the user already holds, not just against the
// amount asked for. Without that, twenty purchases of one slot walk straight
// past a cap of twenty.
func TestPurchaseEnforcesCapCumulatively(t *testing.T) {
	repo := newStubAddonRepo(1000)
	svc := newTestAddonService(repo, nil)
	ctx := context.Background()

	_, err := svc.Purchase(ctx, 42, AddonKindConcurrency, 15, 1)
	require.NoError(t, err)

	balanceAfterFirst := repo.balance

	// 15 + 6 = 21, one past the cap of 20 — even though 6 on its own is fine.
	_, err = svc.Purchase(ctx, 42, AddonKindConcurrency, 6, 1)
	require.Error(t, err)
	require.Equal(t, "ADDON_CAP_EXCEEDED", infraerrors.Reason(err))
	require.InDelta(t, balanceAfterFirst, repo.balance, 1e-9, "a refused order must not be charged")
	require.Equal(t, 15, repo.rows[AddonKindConcurrency].Amount)

	// Exactly filling the cap is still allowed.
	_, err = svc.Purchase(ctx, 42, AddonKindConcurrency, 5, 1)
	require.NoError(t, err)
	require.Equal(t, 20, repo.rows[AddonKindConcurrency].Amount)

	// And nothing more fits after that.
	_, err = svc.Purchase(ctx, 42, AddonKindConcurrency, 1, 1)
	require.Error(t, err)
	require.Equal(t, 20, repo.rows[AddonKindConcurrency].Amount)
}

// A cap breach found inside the transaction takes the debit back with it.
func TestPurchaseRollsBackDebitWhenCapIsBreached(t *testing.T) {
	repo := newStubAddonRepo(1000)
	repo.rows[AddonKindConcurrency] = &UserAddon{
		ID: 1, UserID: 42, Kind: AddonKindConcurrency, Amount: 18, ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	svc := newTestAddonService(repo, nil)

	_, err := svc.Purchase(context.Background(), 42, AddonKindConcurrency, 5, 1)
	require.Error(t, err)
	require.InDelta(t, 1000, repo.balance, 1e-9)
	require.Equal(t, 18, repo.rows[AddonKindConcurrency].Amount)
}

// An order nobody can afford is refused, and refused without leaving an add-on
// behind — the debit and the write share a transaction precisely so that
// "money taken, nothing granted" and "granted, nothing taken" are both
// impossible.
func TestPurchaseRefusesWhenBalanceIsShort(t *testing.T) {
	repo := newStubAddonRepo(5)
	svc := newTestAddonService(repo, nil)

	got, err := svc.Purchase(context.Background(), 42, AddonKindConcurrency, 3, 2) // $12
	require.Error(t, err)
	require.Nil(t, got)
	require.Equal(t, "INSUFFICIENT_BALANCE", infraerrors.Reason(err))
	require.InDelta(t, 5, repo.balance, 1e-9)
	require.Empty(t, repo.rows, "nothing may be granted when the debit is refused")
	require.Equal(t, 1, repo.rollbacks)
}

// A write that fails after the debit takes the money back.
func TestPurchaseRollsBackDebitWhenWriteFails(t *testing.T) {
	repo := newStubAddonRepo(100)
	repo.upsertErr = errors.New("disk on fire")
	svc := newTestAddonService(repo, nil)

	_, err := svc.Purchase(context.Background(), 42, AddonKindConcurrency, 1, 1)
	require.Error(t, err)
	require.InDelta(t, 100, repo.balance, 1e-9, "a failed write must not leave the balance debited")
	require.Empty(t, repo.debits)
}

// Buying again extends the existing row rather than stacking a second one: the
// new term runs from the current expiry, and the amounts add up in place. See
// migrations/228_user_addons.sql for why one row per (user, kind).
func TestPurchaseExtendsRatherThanStacking(t *testing.T) {
	repo := newStubAddonRepo(1000)
	svc := newTestAddonService(repo, nil)
	ctx := context.Background()

	first, err := svc.Purchase(ctx, 42, AddonKindConcurrency, 2, 1)
	require.NoError(t, err)

	second, err := svc.Purchase(ctx, 42, AddonKindConcurrency, 3, 2)
	require.NoError(t, err)

	require.Len(t, repo.rows, 1, "a repeat purchase must not stack a second row")
	require.Equal(t, 5, repo.rows[AddonKindConcurrency].Amount)
	require.Equal(t, 5, second.HeldAfter)
	require.True(t, second.ExpiresAt.After(first.ExpiresAt),
		"the second term must run from the first term's end, not from now")
	require.WithinDuration(t, first.ExpiresAt.AddDate(0, 2, 0), second.ExpiresAt, time.Minute)
}

// A lapsed holding counts for nothing: not against the cap, and not as a date
// to extend from. Back-dating the new term into a window the buyer got no use
// out of would be charging them for time already gone.
func TestPurchaseTreatsLapsedHoldingsAsGone(t *testing.T) {
	repo := newStubAddonRepo(1000)
	repo.rows[AddonKindConcurrency] = &UserAddon{
		ID: 1, UserID: 42, Kind: AddonKindConcurrency, Amount: 20, ExpiresAt: time.Now().Add(-time.Hour),
	}
	svc := newTestAddonService(repo, nil)

	before := time.Now()
	got, err := svc.Purchase(context.Background(), 42, AddonKindConcurrency, 4, 1)
	require.NoError(t, err)

	require.Equal(t, 4, got.HeldAfter, "an expired holding must not count against the cap")
	require.WithinDuration(t, before.AddDate(0, 1, 0), got.ExpiresAt, time.Minute)
}

// Reads decide expiry, not the sweep: a sweep that stalls or is switched off
// must not keep handing out concurrency nobody is paying for.
func TestResolveActiveAddonsDropsLapsedRows(t *testing.T) {
	repo := newStubAddonRepo(0)
	repo.rows[AddonKindConcurrency] = &UserAddon{
		Kind: AddonKindConcurrency, Amount: 6, ExpiresAt: time.Now().Add(time.Hour),
	}
	repo.rows[AddonKindRPM] = &UserAddon{
		Kind: AddonKindRPM, Amount: 300, ExpiresAt: time.Now().Add(-time.Minute),
	}
	svc := newTestAddonService(repo, nil)

	holdings, err := svc.ResolveActiveAddons(context.Background(), 42)
	require.NoError(t, err)

	require.Equal(t, 6, holdings.Concurrency)
	require.NotNil(t, holdings.ConcurrencyExpiresAt)
	require.Zero(t, holdings.RPM, "a lapsed add-on grants nothing")
	require.Nil(t, holdings.RPMExpiresAt)
}

// The catalogue answers prices, caps, and what the caller holds in one call.
func TestCatalogueReportsPricingAndHoldings(t *testing.T) {
	repo := newStubAddonRepo(0)
	repo.rows[AddonKindConcurrency] = &UserAddon{
		Kind: AddonKindConcurrency, Amount: 4, ExpiresAt: time.Now().Add(time.Hour),
	}
	svc := newTestAddonService(repo, nil)

	catalogue, err := svc.Catalogue(context.Background(), 42)
	require.NoError(t, err)

	require.InDelta(t, defaultAddonConcurrencyUnitPrice, catalogue.Pricing.ConcurrencyUnitPrice, 1e-9)
	require.Equal(t, defaultAddonConcurrencyMax, catalogue.Pricing.ConcurrencyMax)
	require.Equal(t, defaultAddonRPMMax, catalogue.Pricing.RPMMax)
	require.Equal(t, rpmAddonStep, catalogue.Pricing.RPMStep)
	require.Equal(t, 4, catalogue.Holdings.Concurrency)
	require.Zero(t, catalogue.Holdings.RPM)
}

// Repricing takes effect on the next purchase, and the cap moves with it.
func TestPurchaseUsesEditedPricing(t *testing.T) {
	repo := newStubAddonRepo(1000)
	settings := newStubAddonSettingRepo()
	pricing := NewAddonPricingService(settings)
	svc := NewAddonService(repo, pricing, nil)
	ctx := context.Background()

	price := 3.5
	limit := 4
	_, err := pricing.Update(ctx, UpdateAddonPricingRequest{ConcurrencyUnitPrice: &price, ConcurrencyMax: &limit})
	require.NoError(t, err)

	got, err := svc.Purchase(ctx, 42, AddonKindConcurrency, 4, 2)
	require.NoError(t, err)
	require.InDelta(t, 28, got.Price, 1e-9) // 4 × 3.5 × 2

	_, err = svc.Purchase(ctx, 42, AddonKindConcurrency, 1, 1)
	require.Error(t, err, "the edited cap binds the next purchase")
}

// A kind priced or capped at zero is off sale rather than free.
func TestPurchaseRefusesWhenKindIsOffSale(t *testing.T) {
	repo := newStubAddonRepo(1000)
	settings := newStubAddonSettingRepo()
	settings.values[SettingAddonConcurrencyMax] = "0"
	svc := NewAddonService(repo, NewAddonPricingService(settings), nil)

	_, err := svc.Purchase(context.Background(), 42, AddonKindConcurrency, 1, 1)
	require.Error(t, err)
	require.Equal(t, "ADDON_NOT_FOR_SALE", infraerrors.Reason(err))
	require.Zero(t, repo.debitCalls)
}

func TestPurchaseResellerPlanDebitsThenAssigns(t *testing.T) {
	plan := &ResellerPlan{ID: 2, Level: 2, Name: "Reseller 2", Price: 150, CreditRate: 0.6, ValidityDays: 365, Enabled: true}
	planRepo := &stubAddonPlanRepo{plans: map[int64]*ResellerPlan{2: plan}}
	repo := newStubAddonRepo(200)
	svc := newTestAddonService(repo, planRepo)

	assignment, err := svc.PurchaseResellerPlan(context.Background(), 42, 2)
	require.NoError(t, err)

	require.Equal(t, plan, assignment.Plan)
	require.Equal(t, []float64{150}, repo.debits, "the price is debited from the buyer's own balance")
	require.InDelta(t, 50, repo.balance, 1e-9)
	// The credit is computed by AssignPlan, not recomputed here: one copy of
	// the arithmetic, so the store and an admin grant can never disagree.
	require.InDelta(t, 90, planRepo.assignedCredit, 1e-9)
	require.Equal(t, 1, planRepo.assignCalls)
	require.Equal(t, 1, repo.txDepth, "the debit and the assignment share one transaction")
}

// Buying a tier twice is a real money bug, not an inconvenience: assignment
// pays out price × credit_rate *every* time, while the expiry runs from now
// rather than extending, so the second payment buys nothing and mints the
// credit again. At a high credit rate that is a balance printer.
func TestPurchaseResellerPlanRefusesWhenOneIsAlreadyActive(t *testing.T) {
	plan := &ResellerPlan{ID: 3, Level: 3, Price: 400, CreditRate: 0.7, ValidityDays: 365, Enabled: true}
	planRepo := &stubAddonPlanRepo{
		plans:      map[int64]*ResellerPlan{3: plan},
		assignment: &ResellerPlanAssignment{Plan: plan, ExpiresAt: time.Now().Add(24 * time.Hour)},
	}
	repo := newStubAddonRepo(10_000)
	svc := newTestAddonService(repo, planRepo)

	got, err := svc.PurchaseResellerPlan(context.Background(), 42, 3)

	require.Error(t, err)
	require.Nil(t, got)
	require.Equal(t, "RESELLER_PLAN_ALREADY_ACTIVE", infraerrors.Reason(err))
	require.Zero(t, planRepo.assignCalls, "a second assignment would pay the credit twice")
	require.Zero(t, repo.debitCalls)
	require.InDelta(t, 10_000, repo.balance, 1e-9)
}

// A lapsed or disabled tier is not an active one: the holder may buy again,
// because this time the credit is being paid for.
func TestPurchaseResellerPlanAllowsRepurchaseOnceLapsed(t *testing.T) {
	plan := &ResellerPlan{ID: 1, Level: 1, Price: 50, CreditRate: 0.5, ValidityDays: 30, Enabled: true}
	for name, held := range map[string]*ResellerPlanAssignment{
		"expired":  {Plan: plan, ExpiresAt: time.Now().Add(-time.Hour)},
		"disabled": {Plan: &ResellerPlan{ID: 1, Price: 50, Enabled: false}, ExpiresAt: time.Now().Add(time.Hour)},
		"none":     nil,
	} {
		t.Run(name, func(t *testing.T) {
			planRepo := &stubAddonPlanRepo{plans: map[int64]*ResellerPlan{1: plan}, assignment: held}
			repo := newStubAddonRepo(100)
			svc := newTestAddonService(repo, planRepo)

			_, err := svc.PurchaseResellerPlan(context.Background(), 42, 1)
			require.NoError(t, err)
			require.Equal(t, 1, planRepo.assignCalls)
			require.InDelta(t, 50, repo.balance, 1e-9)
		})
	}
}

func TestPurchaseResellerPlanRefusesUnknownDisabledAndUnaffordable(t *testing.T) {
	enabled := &ResellerPlan{ID: 1, Level: 1, Price: 50, CreditRate: 0.5, ValidityDays: 30, Enabled: true}
	disabled := &ResellerPlan{ID: 9, Level: 9, Price: 10, ValidityDays: 30, Enabled: false}

	for _, tc := range []struct {
		name    string
		planID  int64
		balance float64
	}{
		{name: "unknown plan", planID: 404, balance: 1000},
		{name: "disabled plan", planID: 9, balance: 1000},
		{name: "cannot afford it", planID: 1, balance: 49.99},
	} {
		t.Run(tc.name, func(t *testing.T) {
			planRepo := &stubAddonPlanRepo{plans: map[int64]*ResellerPlan{1: enabled, 9: disabled}}
			repo := newStubAddonRepo(tc.balance)
			svc := newTestAddonService(repo, planRepo)

			got, err := svc.PurchaseResellerPlan(context.Background(), 42, tc.planID)

			require.Error(t, err)
			require.Nil(t, got)
			require.Zero(t, planRepo.assignCalls, "nothing may be stamped for a rejected purchase")
			require.InDelta(t, tc.balance, repo.balance, 1e-9)
		})
	}
}

// If the assignment fails the debit goes back: taking payment for a tier that
// was never granted is the failure this transaction exists to prevent.
func TestPurchaseResellerPlanRollsBackDebitWhenAssignFails(t *testing.T) {
	plan := &ResellerPlan{ID: 2, Level: 2, Price: 150, CreditRate: 0.6, ValidityDays: 365, Enabled: true}
	planRepo := &stubAddonPlanRepo{
		plans:     map[int64]*ResellerPlan{2: plan},
		assignErr: errors.New("database went away"),
	}
	repo := newStubAddonRepo(200)
	svc := newTestAddonService(repo, planRepo)

	got, err := svc.PurchaseResellerPlan(context.Background(), 42, 2)

	require.Error(t, err)
	require.Nil(t, got)
	require.InDelta(t, 200, repo.balance, 1e-9, "a failed assignment must not keep the money")
	require.Empty(t, repo.debits)
	require.Equal(t, 1, repo.rollbacks)
}
