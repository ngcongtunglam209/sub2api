package service

import (
	"context"
	"time"

	"github.com/shopspring/decimal"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// ResellerPlan is a purchased partnership tier: a one-off payment that returns
// part of itself as balance and opens up what a reseller needs to resell.
//
// Deliberately not a VIP tier. VIP is *earned* by spend and moves with it;
// this is *bought* and stays put until it expires. They look alike today and
// will not for long — this one grows domain quotas and group whitelists, which
// mean nothing to a loyalty ladder.
type ResellerPlan struct {
	ID    int64
	Level int
	Name  string
	Price float64
	// CreditRate is the fraction of Price handed back as balance. 0.6 means
	// pay 100, get 60 to spend.
	//
	// A one-off credit rather than a standing top-up discount: a discount
	// applies to every future payment and compounds invisibly with group and
	// VIP multipliers, while a credit is bounded, visible, and stops.
	CreditRate       float64
	ConcurrencyBonus int
	RPMLimit         int
	MaxDomains       int
	ValidityDays     int
	// AllowedGroupIDs limits which groups this tier may sell codes against.
	// Empty means unrestricted. Resellers pick from groups we define; they
	// never set rates themselves — a rate they controlled would only hand
	// their own customers fewer tokens and earn them nothing.
	AllowedGroupIDs []int64
	Enabled         bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ResellerPlanUpdate carries an admin edit. A nil field means "leave alone",
// so a caller that only wants to switch a tier off does not have to resend
// the price and risk clobbering it with a stale value.
//
// Level and Name are deliberately absent: level is the unique key the ladder
// orders by and the plan assignments point at, and renaming a tier resellers
// have already bought changes what they believe they paid for. Both are
// migrations, not edits.
type ResellerPlanUpdate struct {
	Price            *float64
	CreditRate       *float64
	ConcurrencyBonus *int
	RPMLimit         *int
	MaxDomains       *int
	ValidityDays     *int
	AllowedGroupIDs  *[]int64
	Enabled          *bool
}

// applyResellerPlanUpdate folds an edit onto the stored plan.
func applyResellerPlanUpdate(base ResellerPlan, in ResellerPlanUpdate) ResellerPlan {
	if in.Price != nil {
		base.Price = *in.Price
	}
	if in.CreditRate != nil {
		base.CreditRate = *in.CreditRate
	}
	if in.ConcurrencyBonus != nil {
		base.ConcurrencyBonus = *in.ConcurrencyBonus
	}
	if in.RPMLimit != nil {
		base.RPMLimit = *in.RPMLimit
	}
	if in.MaxDomains != nil {
		base.MaxDomains = *in.MaxDomains
	}
	if in.ValidityDays != nil {
		base.ValidityDays = *in.ValidityDays
	}
	if in.AllowedGroupIDs != nil {
		// Copy: the slice comes straight off a decoded request body, and storing
		// the caller's backing array would let a later mutation of it change what
		// we think we wrote.
		ids := make([]int64, len(*in.AllowedGroupIDs))
		copy(ids, *in.AllowedGroupIDs)
		base.AllowedGroupIDs = ids
	}
	if in.Enabled != nil {
		base.Enabled = *in.Enabled
	}
	return base
}

// validateResellerPlanFields rejects a tier that would misbehave once sold.
//
// The bounds are not decoration. A credit rate above 1 pays a reseller more
// balance than they handed over, which mints money on every purchase; a
// validity of zero or less produces a plan that is already expired the instant
// it is assigned, so the buyer pays and gets nothing.
func validateResellerPlanFields(p ResellerPlan) error {
	if p.Price < 0 {
		return infraerrors.BadRequest("INVALID_RESELLER_PLAN_PRICE", "price must not be negative")
	}
	if p.CreditRate < 0 || p.CreditRate > 1 {
		return infraerrors.BadRequest("INVALID_RESELLER_PLAN_CREDIT_RATE", "credit rate must be between 0 and 1")
	}
	if p.ValidityDays <= 0 {
		return infraerrors.BadRequest("INVALID_RESELLER_PLAN_VALIDITY", "validity days must be greater than zero")
	}
	if p.ConcurrencyBonus < 0 {
		return infraerrors.BadRequest("INVALID_RESELLER_PLAN_CONCURRENCY", "concurrency bonus must not be negative")
	}
	if p.RPMLimit < 0 {
		return infraerrors.BadRequest("INVALID_RESELLER_PLAN_RPM", "rpm limit must not be negative")
	}
	if p.MaxDomains < 0 {
		return infraerrors.BadRequest("INVALID_RESELLER_PLAN_MAX_DOMAINS", "max domains must not be negative")
	}
	return nil
}

// ResellerPlanAssignment is what a user currently holds.
type ResellerPlanAssignment struct {
	Plan      *ResellerPlan
	ExpiresAt time.Time
}

// Active reports whether the assignment still confers anything.
func (a *ResellerPlanAssignment) Active(now time.Time) bool {
	return a != nil && a.Plan != nil && a.Plan.Enabled && now.Before(a.ExpiresAt)
}

type ResellerPlanRepository interface {
	List(ctx context.Context) ([]*ResellerPlan, error)
	GetByID(ctx context.Context, id int64) (*ResellerPlan, error)

	// Update writes the mutable columns of an already-validated plan. It takes
	// the whole row rather than a patch so the merge-and-validate step lives in
	// one place in the service, where the bounds are checked, instead of being
	// half in the repository where they would be easy to skip.
	Update(ctx context.Context, plan *ResellerPlan) (*ResellerPlan, error)

	// AssignToUser stamps the plan on the user and credits the balance in one
	// transaction. Splitting the two is how a crash between them either gives
	// a reseller a tier they never paid for or takes payment without the
	// credit — both need a human to untangle.
	AssignToUser(ctx context.Context, userID int64, plan *ResellerPlan, expiresAt time.Time, creditAmount float64) error

	// GetUserAssignment returns nil when the user holds no plan. An expired
	// plan is still returned; the caller decides, so callers that only report
	// status can show "expired" rather than "never had one".
	GetUserAssignment(ctx context.Context, userID int64) (*ResellerPlanAssignment, error)

	ClearUserAssignment(ctx context.Context, userID int64) error
}

// calculateResellerCredit converts a plan price into the balance handed back.
//
// decimal rather than float64 for the same reason calculateCreditedBalance
// uses it: this number is money, and 0.7 × 400 in binary floating point is not
// 280.
func calculateResellerCredit(price, creditRate float64) float64 {
	if price <= 0 || creditRate <= 0 {
		return 0
	}
	credited, _ := decimal.NewFromFloat(price).
		Mul(decimal.NewFromFloat(creditRate)).
		Round(2).
		Float64()
	return credited
}
