package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// ResellerPlanService owns assigning purchased reseller tiers and answering
// what a tier entitles its holder to.
//
// Assignment is admin-driven for now: the plan is paid for out of band and
// stamped on afterwards. Selling it through checkout would mean a third
// order type in the payment path, which is the code least worth disturbing
// for the sake of the first few resellers.
type ResellerPlanService struct {
	repo ResellerPlanRepository
}

func NewResellerPlanService(repo ResellerPlanRepository) *ResellerPlanService {
	return &ResellerPlanService{repo: repo}
}

// SetResellerPlanResolver injects the plan lookup the auth snapshot uses for
// the concurrency bonus.
//
// Set after construction because the plan service is built later in the
// dependency graph than the API key service, and threading it through every
// existing NewAPIKeyService caller to reach one optional perk is not worth the
// churn. Left unset, the bonus is simply not applied — which is the behaviour
// before reseller plans existed.
func (s *APIKeyService) SetResellerPlanResolver(resolver ResellerPlanResolver) {
	if s != nil {
		s.resellerPlanResolver = resolver
	}
}

func (s *ResellerPlanService) List(ctx context.Context) ([]*ResellerPlan, error) {
	return s.repo.List(ctx)
}

// Update edits a tier's terms.
//
// Holders are not re-stamped: their expiry and credited balance were settled
// when they bought in, and repricing a tier must not retroactively shorten a
// plan somebody already paid for. The new terms apply from the next assignment.
func (s *ResellerPlanService) Update(ctx context.Context, id int64, in ResellerPlanUpdate) (*ResellerPlan, error) {
	if id <= 0 {
		return nil, infraerrors.BadRequest("INVALID_RESELLER_PLAN", "invalid reseller plan id")
	}

	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, infraerrors.NotFound("RESELLER_PLAN_NOT_FOUND", "reseller plan not found")
	}

	// Merge onto the stored row, then validate the result rather than the patch:
	// a field left alone still has to be legal alongside the ones that changed.
	candidate := applyResellerPlanUpdate(*current, in)
	if err := validateResellerPlanFields(candidate); err != nil {
		return nil, err
	}

	return s.repo.Update(ctx, &candidate)
}

// GetPlan returns one tier, or nil when no such tier exists.
//
// Exported for the self-service store, which has to read the price before it
// can debit it. Deliberately a read: the store must not be able to reach the
// repository's write side.
func (s *ResellerPlanService) GetPlan(ctx context.Context, id int64) (*ResellerPlan, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	if id <= 0 {
		return nil, infraerrors.BadRequest("INVALID_RESELLER_PLAN", "invalid reseller plan id")
	}
	return s.repo.GetByID(ctx, id)
}

// AssignPlan grants a purchased tier: stamps it on the user, sets its expiry,
// and credits the agreed share of the price back as balance.
//
// Safe to call inside a caller's transaction: AssignToUser joins one carried
// on the context when there is one. That is what lets the self-service store
// debit the price and hand the tier over atomically without a second copy of
// the expiry and credit arithmetic.
func (s *ResellerPlanService) AssignPlan(ctx context.Context, userID, planID int64) (*ResellerPlanAssignment, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "a reseller plan must be assigned to a user")
	}

	plan, err := s.repo.GetByID(ctx, planID)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, infraerrors.NotFound("RESELLER_PLAN_NOT_FOUND", "reseller plan not found")
	}
	if !plan.Enabled {
		return nil, infraerrors.BadRequest("RESELLER_PLAN_DISABLED", "this reseller plan is no longer offered")
	}

	// Expiry runs from now, not from any previous plan's end date. Stacking
	// would let a reseller buy the cheapest tier repeatedly and accumulate
	// years, and the credit is paid out per assignment regardless.
	expiresAt := time.Now().AddDate(0, 0, plan.ValidityDays)
	credit := calculateResellerCredit(plan.Price, plan.CreditRate)

	if err := s.repo.AssignToUser(ctx, userID, plan, expiresAt, credit); err != nil {
		return nil, err
	}

	return &ResellerPlanAssignment{Plan: plan, ExpiresAt: expiresAt}, nil
}

// ResolveForUser returns the user's assignment, or nil when they hold none.
func (s *ResellerPlanService) ResolveForUser(ctx context.Context, userID int64) (*ResellerPlanAssignment, error) {
	if s == nil || s.repo == nil || userID <= 0 {
		return nil, nil
	}
	return s.repo.GetUserAssignment(ctx, userID)
}

// MaxDomainsForUser reports how many custom domains the user may register.
//
// Zero for anyone without an active plan, which is what keeps the domain
// allowlist from being something an ordinary account can grow.
func (s *ResellerPlanService) MaxDomainsForUser(ctx context.Context, userID int64) (int, error) {
	assignment, err := s.ResolveForUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	if !assignment.Active(time.Now()) {
		return 0, nil
	}
	return assignment.Plan.MaxDomains, nil
}

// Revoke removes a plan, e.g. on refund or breach. The credited balance is
// deliberately left alone: it was spent into circulation as redeem codes the
// moment it was granted, and clawing it back would bounce codes already sold
// to people who did nothing wrong.
func (s *ResellerPlanService) Revoke(ctx context.Context, userID int64) error {
	return s.repo.ClearUserAssignment(ctx, userID)
}
