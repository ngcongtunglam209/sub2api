package service

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
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
