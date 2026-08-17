package service

import (
	"context"
	"fmt"
	"math"
	"strconv"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// Add-on pricing lives in the settings KV alongside the other money tunables
// (see SettingBalanceRechargeMult) rather than in a table of its own.
//
// Six scalars do not need a schema, a repository, and a migration each time an
// operator wants to try $3 a slot. A table would also imply a history nobody
// keeps: repricing does not re-bill anyone, exactly as repricing a reseller
// tier leaves existing holders alone.
const (
	SettingAddonConcurrencyUnitPrice = "ADDON_CONCURRENCY_UNIT_PRICE"
	SettingAddonConcurrencyMax       = "ADDON_CONCURRENCY_MAX"
	SettingAddonRPMUnitPrice         = "ADDON_RPM_UNIT_PRICE"
	SettingAddonRPMMax               = "ADDON_RPM_MAX"
)

// Defaults ship a working store on a fresh install: unset keys price the
// catalogue rather than disabling it.
//
// The concurrency cap is the number that matters. The whole pool's ceiling is
// "usable accounts × 3" and drops to single digits when accounts are throttled
// — the same scarcity that keeps reseller concurrency_bonus deliberately mean.
// 20 slots is already more than one account should be able to reserve; it is a
// backstop, not a target.
const (
	defaultAddonConcurrencyUnitPrice = 2.0
	defaultAddonConcurrencyMax       = 20
	defaultAddonRPMUnitPrice         = 1.0
	defaultAddonRPMMax               = 600
)

// AddonPricing is the catalogue: what a unit costs and how much of it one user
// may ever hold.
type AddonPricing struct {
	ConcurrencyUnitPrice float64 `json:"concurrency_unit_price"`
	ConcurrencyMax       int     `json:"concurrency_max"`
	// RPMUnitPrice buys RPMStep RPM for one month.
	RPMUnitPrice float64 `json:"rpm_unit_price"`
	RPMStep      int     `json:"rpm_step"`
	RPMMax       int     `json:"rpm_max"`
	MinMonths    int     `json:"min_months"`
	MaxMonths    int     `json:"max_months"`
}

// UnitPrice returns the per-unit-per-month price for one kind.
func (p AddonPricing) UnitPrice(kind AddonKind) float64 {
	switch kind {
	case AddonKindConcurrency:
		return p.ConcurrencyUnitPrice
	case AddonKindRPM:
		return p.RPMUnitPrice
	default:
		return 0
	}
}

// Cap returns the most one user may hold of one kind.
func (p AddonPricing) Cap(kind AddonKind) int {
	switch kind {
	case AddonKindConcurrency:
		return p.ConcurrencyMax
	case AddonKindRPM:
		return p.RPMMax
	default:
		return 0
	}
}

// UpdateAddonPricingRequest is an admin edit. Nil means "leave alone", so an
// operator raising one price does not have to resend the caps and risk writing
// back a value they read minutes ago.
type UpdateAddonPricingRequest struct {
	ConcurrencyUnitPrice *float64 `json:"concurrency_unit_price"`
	ConcurrencyMax       *int     `json:"concurrency_max"`
	RPMUnitPrice         *float64 `json:"rpm_unit_price"`
	RPMMax               *int     `json:"rpm_max"`
}

// AddonPricingService reads and writes the catalogue.
type AddonPricingService struct {
	settingRepo SettingRepository
}

func NewAddonPricingService(settingRepo SettingRepository) *AddonPricingService {
	return &AddonPricingService{settingRepo: settingRepo}
}

// Get returns the current catalogue, falling back to the defaults for any key
// an operator has never touched.
//
// A read failure is not swallowed: pricing the store from zeroes would sell
// concurrency for nothing.
func (s *AddonPricingService) Get(ctx context.Context) (AddonPricing, error) {
	if s == nil || s.settingRepo == nil {
		return defaultAddonPricing(), nil
	}
	vals, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingAddonConcurrencyUnitPrice,
		SettingAddonConcurrencyMax,
		SettingAddonRPMUnitPrice,
		SettingAddonRPMMax,
	})
	if err != nil {
		return AddonPricing{}, fmt.Errorf("get addon pricing settings: %w", err)
	}
	return parseAddonPricing(vals), nil
}

// Update writes an operator's edit.
//
// Every bound is checked before the write rather than clamped after it: a
// price of zero would give concurrency away, and a negative cap would refuse
// every purchase while looking like a typo nobody notices until support asks.
func (s *AddonPricingService) Update(ctx context.Context, req UpdateAddonPricingRequest) (AddonPricing, error) {
	if s == nil || s.settingRepo == nil {
		return AddonPricing{}, infraerrors.InternalServer("ADDON_PRICING_UNAVAILABLE", "add-on pricing is not configured")
	}

	m := make(map[string]string, 4)
	if req.ConcurrencyUnitPrice != nil {
		if err := validateAddonUnitPrice(*req.ConcurrencyUnitPrice); err != nil {
			return AddonPricing{}, err
		}
		m[SettingAddonConcurrencyUnitPrice] = strconv.FormatFloat(*req.ConcurrencyUnitPrice, 'f', -1, 64)
	}
	if req.RPMUnitPrice != nil {
		if err := validateAddonUnitPrice(*req.RPMUnitPrice); err != nil {
			return AddonPricing{}, err
		}
		m[SettingAddonRPMUnitPrice] = strconv.FormatFloat(*req.RPMUnitPrice, 'f', -1, 64)
	}
	if req.ConcurrencyMax != nil {
		if err := validateAddonCap(*req.ConcurrencyMax); err != nil {
			return AddonPricing{}, err
		}
		m[SettingAddonConcurrencyMax] = strconv.Itoa(*req.ConcurrencyMax)
	}
	if req.RPMMax != nil {
		if err := validateAddonCap(*req.RPMMax); err != nil {
			return AddonPricing{}, err
		}
		// A cap that is not a whole number of blocks can never be reached: the
		// last purchase that would fit is refused for being a partial block, so
		// the advertised ceiling is a number no customer can buy up to.
		if *req.RPMMax%rpmAddonStep != 0 {
			return AddonPricing{}, infraerrors.BadRequest("INVALID_ADDON_CAP",
				fmt.Sprintf("rpm cap must be a multiple of %d", rpmAddonStep))
		}
		m[SettingAddonRPMMax] = strconv.Itoa(*req.RPMMax)
	}

	if len(m) > 0 {
		if err := s.settingRepo.SetMultiple(ctx, m); err != nil {
			return AddonPricing{}, fmt.Errorf("update addon pricing settings: %w", err)
		}
	}
	return s.Get(ctx)
}

// Lowering a cap deliberately does not claw anything back from users already
// over it: they paid for what they hold, and it lapses on its own. The new cap
// binds the next purchase, which is the same posture reseller plan repricing
// takes toward existing holders.
func validateAddonCap(limit int) error {
	if limit < 0 {
		return infraerrors.BadRequest("INVALID_ADDON_CAP", "cap must not be negative")
	}
	return nil
}

func validateAddonUnitPrice(price float64) error {
	if math.IsNaN(price) || math.IsInf(price, 0) || price <= 0 {
		return infraerrors.BadRequest("INVALID_ADDON_UNIT_PRICE", "unit price must be greater than zero")
	}
	return nil
}

func defaultAddonPricing() AddonPricing {
	return AddonPricing{
		ConcurrencyUnitPrice: defaultAddonConcurrencyUnitPrice,
		ConcurrencyMax:       defaultAddonConcurrencyMax,
		RPMUnitPrice:         defaultAddonRPMUnitPrice,
		RPMStep:              rpmAddonStep,
		RPMMax:               defaultAddonRPMMax,
		MinMonths:            minAddonMonths,
		MaxMonths:            maxAddonMonths,
	}
}

// parseAddonPricing folds stored values onto the defaults.
//
// An unparseable or absent value falls back to the default rather than to
// zero: a corrupted key must not quietly turn the store into a giveaway.
func parseAddonPricing(vals map[string]string) AddonPricing {
	pricing := defaultAddonPricing()
	if v := parseAddonFloat(vals[SettingAddonConcurrencyUnitPrice]); v > 0 {
		pricing.ConcurrencyUnitPrice = v
	}
	if v := parseAddonFloat(vals[SettingAddonRPMUnitPrice]); v > 0 {
		pricing.RPMUnitPrice = v
	}
	// Caps read >= 0 because 0 is a meaningful setting: it takes the kind off
	// sale without deleting anybody's pricing.
	if v, ok := parseAddonInt(vals[SettingAddonConcurrencyMax]); ok && v >= 0 {
		pricing.ConcurrencyMax = v
	}
	if v, ok := parseAddonInt(vals[SettingAddonRPMMax]); ok && v >= 0 {
		pricing.RPMMax = v
	}
	return pricing
}

func parseAddonFloat(raw string) float64 {
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

func parseAddonInt(raw string) (int, bool) {
	if raw == "" {
		return 0, false
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return v, true
}
