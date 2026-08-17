package service

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// AddonKind names a resource a user may buy for themselves.
//
// A string rather than an enum table: adding a third sellable resource should
// be a constant and a price, not a migration.
type AddonKind string

const (
	AddonKindConcurrency AddonKind = "concurrency"
	AddonKindRPM         AddonKind = "rpm"
)

// AddonKinds lists every sellable kind, in catalogue order.
func AddonKinds() []AddonKind {
	return []AddonKind{AddonKindConcurrency, AddonKindRPM}
}

// Valid reports whether kind is something we actually sell.
func (k AddonKind) Valid() bool {
	return k == AddonKindConcurrency || k == AddonKindRPM
}

const (
	// minAddonMonths / maxAddonMonths bound one purchase.
	//
	// The ceiling is not squeamishness about large orders: months multiplies the
	// price *and* the expiry, so an unbounded value is both an overflow-shaped
	// arithmetic input and a way to hold scarce concurrency for a decade at
	// today's price. A year is long enough that nobody is inconvenienced, and
	// short enough that a repricing eventually reaches everyone.
	minAddonMonths = 1
	maxAddonMonths = 12

	// rpmAddonStep is the granularity RPM is sold in: $unit buys this many RPM
	// for a month. Purchases must be a whole number of steps, so the price is
	// always an exact multiple of the unit and never a fraction of a cent.
	//
	// Not a setting: it is the shape of the product, not a knob. Changing what
	// a unit *costs* is a setting; changing what a unit *is* changes what every
	// stored amount means.
	rpmAddonStep = 30
)

// UserAddon is what a user currently holds of one kind.
//
// One row per (user, kind): repeat purchases extend it rather than stacking
// new rows. See migrations/228_user_addons.sql for why — briefly, the auth
// snapshot reads this on the hot path and one row answers it without a SUM,
// and a cap checked against a single total cannot be walked past by a caller
// who finds an aggregation the check forgot.
type UserAddon struct {
	ID        int64
	UserID    int64
	Kind      AddonKind
	Amount    int
	ExpiresAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Active reports whether the add-on still counts.
//
// Resolved on read rather than trusted to the expiry sweep: a sweep that
// stalls or gets switched off must not keep handing out concurrency.
func (a *UserAddon) Active(now time.Time) bool {
	return a != nil && a.Amount > 0 && now.Before(a.ExpiresAt)
}

// AddonHoldings is the resolved, still-valid total per kind.
//
// Expired add-ons are already filtered out here, so every consumer — the auth
// snapshot, the cap check, the catalogue endpoint — sees the same number and
// none of them has to remember to compare against `now` itself.
type AddonHoldings struct {
	Concurrency          int
	ConcurrencyExpiresAt *time.Time
	RPM                  int
	RPMExpiresAt         *time.Time
}

// Amount returns the active amount held of one kind.
func (h AddonHoldings) Amount(kind AddonKind) int {
	switch kind {
	case AddonKindConcurrency:
		return h.Concurrency
	case AddonKindRPM:
		return h.RPM
	default:
		return 0
	}
}

// ExpiresAt returns when the held amount of one kind lapses, or nil.
func (h AddonHoldings) ExpiresAt(kind AddonKind) *time.Time {
	switch kind {
	case AddonKindConcurrency:
		return h.ConcurrencyExpiresAt
	case AddonKindRPM:
		return h.RPMExpiresAt
	default:
		return nil
	}
}

// AddonResolver is the slice of the add-on service the auth snapshot needs.
//
// Mirrors ResellerPlanResolver deliberately: both are injected into
// APIKeyService after construction, and both leave the plain user limit alone
// when they fail.
type AddonResolver interface {
	ResolveActiveAddons(ctx context.Context, userID int64) (AddonHoldings, error)
}

// AddonPurchase is one accepted order, priced and bounded.
type AddonPurchase struct {
	Kind AddonKind
	// Amount is in the kind's own unit: concurrency slots, or RPM.
	Amount int
	Months int
	// Price is what the balance is debited, already rounded to cents.
	Price float64
	// HeldAfter is the total the user will hold once this is applied. Carried
	// on the result so the caller can show it without a second round trip.
	HeldAfter int
	ExpiresAt time.Time
}

// UserAddonRepository stores what users have bought.
//
// The purchase methods are deliberately small and composable rather than one
// do-everything call: the policy (caps, bounds, price) lives in the service
// where it is testable without a database, and the repository only supplies
// the pieces that must touch one transaction together.
type UserAddonRepository interface {
	// ListByUser returns every stored add-on for a user, expired ones included.
	// The caller decides what is still valid — see UserAddon.Active.
	ListByUser(ctx context.Context, userID int64) ([]*UserAddon, error)

	// LockByUserKind reads one add-on FOR UPDATE so a concurrent purchase of
	// the same kind cannot read the same "already holds" figure and let two
	// orders past a cap that only had room for one. Returns nil when the user
	// holds none of that kind.
	//
	// Two *first* purchases race with nothing to lock; the unique index on
	// (user_id, kind) catches that one, and because the debit shares the
	// transaction the loser's money goes back with the rollback.
	LockByUserKind(ctx context.Context, userID int64, kind AddonKind) (*UserAddon, error)

	// Upsert writes the merged add-on for one (user, kind).
	Upsert(ctx context.Context, userID int64, kind AddonKind, amount int, expiresAt time.Time) (*UserAddon, error)

	// DebitBalanceGuarded takes `amount` off a user's balance, refusing rather
	// than overdrawing.
	//
	// Deliberately not UserRepository.DeductBalance: that one falls back to an
	// unconditional write and can drive a balance negative, which is right for
	// usage billing — a request already streaming cannot be refused — and wrong
	// here, where refusing is exactly what should happen.
	DebitBalanceGuarded(ctx context.Context, userID int64, amount float64) error

	// RunInTx runs fn with a transaction bound to the context it passes on.
	// Repository calls made with that context join the transaction.
	RunInTx(ctx context.Context, fn func(ctx context.Context) error) error

	// ListExpiredAddonUserIDs backs the housekeeping sweep.
	ListExpiredAddonUserIDs(ctx context.Context, now time.Time, limit int) ([]int64, error)

	// DeleteExpiredAddons removes lapsed rows for the given users and reports
	// how many went. It re-checks the expiry itself so a row renewed between
	// the list and the delete is not thrown away.
	DeleteExpiredAddons(ctx context.Context, now time.Time, userIDs []int64) (int, error)
}

// calculateAddonPrice prices one order.
//
// decimal rather than float64, for the same reason calculateResellerCredit
// uses it: units × unit price × months is money, and 3 × 0.1 in binary
// floating point is not 0.3.
//
// `units` is the number of priced units, not the raw amount — RPM is sold in
// blocks of rpmAddonStep, so 90 RPM is three units, not ninety.
func calculateAddonPrice(units int, unitPrice float64, months int) float64 {
	if units <= 0 || unitPrice <= 0 || months <= 0 {
		return 0
	}
	price, _ := decimal.NewFromInt(int64(units)).
		Mul(decimal.NewFromFloat(unitPrice)).
		Mul(decimal.NewFromInt(int64(months))).
		Round(2).
		Float64()
	return price
}

// addonPricedUnits converts a requested amount into billable units, refusing
// an amount that does not divide into whole units.
//
// A partial block would have to be rounded somewhere, and every rounding of a
// price is a place a customer and an invoice can disagree. Refusing is cheaper
// to explain than either direction of rounding.
func addonPricedUnits(kind AddonKind, amount int) (int, error) {
	if amount <= 0 {
		return 0, infraerrors.BadRequest("INVALID_ADDON_AMOUNT", "amount must be greater than zero")
	}
	switch kind {
	case AddonKindConcurrency:
		return amount, nil
	case AddonKindRPM:
		if amount%rpmAddonStep != 0 {
			return 0, infraerrors.BadRequest("INVALID_ADDON_AMOUNT",
				fmt.Sprintf("rpm must be bought in multiples of %d", rpmAddonStep))
		}
		return amount / rpmAddonStep, nil
	default:
		return 0, infraerrors.BadRequest("INVALID_ADDON_KIND", "unknown add-on kind")
	}
}

// validateAddonMonths bounds the term of one purchase.
func validateAddonMonths(months int) error {
	if months < minAddonMonths || months > maxAddonMonths {
		return infraerrors.BadRequest("INVALID_ADDON_MONTHS",
			fmt.Sprintf("months must be between %d and %d", minAddonMonths, maxAddonMonths))
	}
	return nil
}

// extendAddonExpiry is where "buying again extends" is actually decided.
//
// The new term runs from the current expiry when the user still holds
// something, and from now when they do not. Extending from `now` instead would
// silently shorten what they already paid for; extending from a *lapsed*
// expiry would back-date the new term into a window they got no use out of.
func extendAddonExpiry(current *UserAddon, now time.Time, months int) time.Time {
	from := now
	if current.Active(now) && current.ExpiresAt.After(now) {
		from = current.ExpiresAt
	}
	return from.AddDate(0, months, 0)
}
