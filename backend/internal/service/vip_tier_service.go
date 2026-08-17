package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/ent/viptier"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// VIPTier is a configured tier as the admin API sees it.
//
// Concurrency and RPMLimit are both *addends* over what the user already holds,
// not ceilings. The two Unlimited flags exist because an addend cannot express
// "no ceiling": adding 0 means "add nothing", which is a legitimate tier, so
// the exemption needs a field of its own.
type VIPTier struct {
	ID                   int64
	Level                int
	Name                 string
	MinSpendUSD          float64
	RateMultiplier       float64
	Concurrency          int
	RPMLimit             int
	UnlimitedConcurrency bool
	UnlimitedRPM         bool
	GraceDays            int
	BadgeColor           string
	Enabled              bool
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// VIPTierInput carries a create or update from the admin API. Nil fields on
// update mean "leave alone".
type VIPTierInput struct {
	Level                *int
	Name                 *string
	MinSpendUSD          *float64
	RateMultiplier       *float64
	Concurrency          *int
	RPMLimit             *int
	UnlimitedConcurrency *bool
	UnlimitedRPM         *bool
	GraceDays            *int
	BadgeColor           *string
	Enabled              *bool
}

// VIPBenefits is what a user's active tier grants on top of their own limits.
//
// Returned as one value rather than a getter per perk so the auth snapshot
// resolves the tier in a single query: it is on the hot path, and a second
// round trip per request buys nothing.
type VIPBenefits struct {
	Concurrency          int
	RPM                  int
	UnlimitedConcurrency bool
	UnlimitedRPM         bool
}

// VIPStatus is what a user sees about their own standing.
type VIPStatus struct {
	Tier            *VIPTier
	NextTier        *VIPTier
	QualifyingSpend float64
	TotalPaidUSD    float64
	SpendToNextTier float64
	ExpiresAt       *time.Time
	Locked          bool
}

// VIPTierService is the admin/user facing side of the tier system. Grading
// itself lives on the payment path, because a tier is earned by an order
// completing, not by anyone calling an endpoint.
type VIPTierService struct {
	entClient   *dbent.Client
	invalidator APIKeyAuthCacheInvalidator
}

func NewVIPTierService(entClient *dbent.Client) *VIPTierService {
	return &VIPTierService{entClient: entClient}
}

// SetAuthCacheInvalidator wires snapshot invalidation for admin edits, which
// change the concurrency floor cached against every affected user.
func (s *VIPTierService) SetAuthCacheInvalidator(invalidator APIKeyAuthCacheInvalidator) {
	if s == nil {
		return
	}
	s.invalidator = invalidator
}

func vipTierFromEnt(t *dbent.VIPTier) *VIPTier {
	if t == nil {
		return nil
	}
	return &VIPTier{
		ID:                   t.ID,
		Level:                t.Level,
		Name:                 t.Name,
		MinSpendUSD:          t.MinSpendUsd,
		RateMultiplier:       t.RateMultiplier,
		Concurrency:          t.Concurrency,
		RPMLimit:             t.RpmLimit,
		UnlimitedConcurrency: t.UnlimitedConcurrency,
		UnlimitedRPM:         t.UnlimitedRpm,
		GraceDays:            t.GraceDays,
		BadgeColor:           t.BadgeColor,
		Enabled:              t.Enabled,
		CreatedAt:            t.CreatedAt,
		UpdatedAt:            t.UpdatedAt,
	}
}

// List returns every tier, lowest level first.
func (s *VIPTierService) List(ctx context.Context) ([]VIPTier, error) {
	if s == nil || s.entClient == nil {
		return nil, infraerrors.InternalServer("VIP_UNAVAILABLE", "vip tiers are unavailable")
	}
	rows, err := s.entClient.VIPTier.Query().Order(dbent.Asc(viptier.FieldLevel)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list vip tiers: %w", err)
	}
	out := make([]VIPTier, 0, len(rows))
	for _, row := range rows {
		out = append(out, *vipTierFromEnt(row))
	}
	return out, nil
}

// validateVIPLadder rejects a ladder that would pay users to spend less.
//
// A higher level must cost more and discount more. Without this an admin can
// leave VIP2 cheaper and more generous than VIP3, and every VIP3 customer is
// then better off having spent less â€” the ladder stops being a ladder.
func validateVIPLadder(tiers []VIPTier) error {
	sorted := make([]VIPTier, len(tiers))
	copy(sorted, tiers)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Level < sorted[j].Level })

	for i := 1; i < len(sorted); i++ {
		prev, cur := sorted[i-1], sorted[i]
		if cur.Level == prev.Level {
			return infraerrors.BadRequest("INVALID_VIP_LADDER", fmt.Sprintf("level %d is used twice", cur.Level))
		}
		if cur.MinSpendUSD <= prev.MinSpendUSD {
			return infraerrors.BadRequest("INVALID_VIP_LADDER",
				fmt.Sprintf("%s must require more spend than %s", cur.Name, prev.Name))
		}
		if cur.RateMultiplier > prev.RateMultiplier {
			return infraerrors.BadRequest("INVALID_VIP_LADDER",
				fmt.Sprintf("%s must not bill higher than %s", cur.Name, prev.Name))
		}
		// The perks have to climb with the price for the same reason the discount
		// does. Left unchecked, a VIP4 granting less concurrency than VIP3 leaves
		// every VIP4 customer worse off for having spent more — which is how a
		// tier of 32 slots sitting above tiers of 1, 2 and 3 goes unnoticed until
		// somebody reads the table.
		if vipBenefitRank(cur.Concurrency, cur.UnlimitedConcurrency) < vipBenefitRank(prev.Concurrency, prev.UnlimitedConcurrency) {
			return infraerrors.BadRequest("INVALID_VIP_LADDER",
				fmt.Sprintf("%s must not grant less concurrency than %s", cur.Name, prev.Name))
		}
		if vipBenefitRank(cur.RPMLimit, cur.UnlimitedRPM) < vipBenefitRank(prev.RPMLimit, prev.UnlimitedRPM) {
			return infraerrors.BadRequest("INVALID_VIP_LADDER",
				fmt.Sprintf("%s must not grant less RPM than %s", cur.Name, prev.Name))
		}
	}
	return nil
}

// vipBenefitRank orders a grant for ladder comparison, sorting an exemption
// above every finite amount.
//
// Comparing the raw numbers would rank an unlimited tier by whatever its unused
// addend happens to be, so an unlimited VIP4 with rpm_limit 0 would read as
// granting less than a VIP3 granting 60.
func vipBenefitRank(value int, unlimited bool) int {
	if unlimited {
		return math.MaxInt
	}
	return value
}

func validateVIPTierFields(t VIPTier) error {
	if t.Level <= 0 {
		return infraerrors.BadRequest("INVALID_INPUT", "level must be positive")
	}
	if strings.TrimSpace(t.Name) == "" {
		return infraerrors.BadRequest("INVALID_INPUT", "name is required")
	}
	if t.MinSpendUSD < 0 {
		return infraerrors.BadRequest("INVALID_INPUT", "min_spend_usd must not be negative")
	}
	// Above 1 would bill a paying customer more than the base rate; 0 would
	// make every request free, which is a footgun rather than a tier.
	if t.RateMultiplier <= 0 || t.RateMultiplier > 1 {
		return infraerrors.BadRequest("INVALID_INPUT", "rate_multiplier must be greater than 0 and at most 1")
	}
	// Positive, not non-negative: a tier granting no concurrency at all is
	// indistinguishable from having no tier, and 0 is the sentinel the auth
	// snapshot reads as "no ceiling" — storing it here would be ambiguous. Use
	// unlimited_concurrency for the exemption instead.
	if t.Concurrency <= 0 {
		return infraerrors.BadRequest("INVALID_INPUT", "concurrency must be positive")
	}
	// RPM differs: granting no extra RPM is a real tier, so 0 is allowed and
	// means "add nothing".
	if t.RPMLimit < 0 {
		return infraerrors.BadRequest("INVALID_INPUT", "rpm_limit must not be negative")
	}
	if t.GraceDays <= 0 {
		return infraerrors.BadRequest("INVALID_INPUT", "grace_days must be positive")
	}
	return nil
}

func applyVIPTierInput(base VIPTier, in VIPTierInput) VIPTier {
	if in.Level != nil {
		base.Level = *in.Level
	}
	if in.Name != nil {
		base.Name = strings.TrimSpace(*in.Name)
	}
	if in.MinSpendUSD != nil {
		base.MinSpendUSD = *in.MinSpendUSD
	}
	if in.RateMultiplier != nil {
		base.RateMultiplier = *in.RateMultiplier
	}
	if in.Concurrency != nil {
		base.Concurrency = *in.Concurrency
	}
	if in.RPMLimit != nil {
		base.RPMLimit = *in.RPMLimit
	}
	if in.UnlimitedConcurrency != nil {
		base.UnlimitedConcurrency = *in.UnlimitedConcurrency
	}
	if in.UnlimitedRPM != nil {
		base.UnlimitedRPM = *in.UnlimitedRPM
	}
	if in.GraceDays != nil {
		base.GraceDays = *in.GraceDays
	}
	if in.BadgeColor != nil {
		base.BadgeColor = strings.TrimSpace(*in.BadgeColor)
	}
	if in.Enabled != nil {
		base.Enabled = *in.Enabled
	}
	return base
}

// Create adds a tier, rejecting anything that breaks the ladder ordering.
func (s *VIPTierService) Create(ctx context.Context, in VIPTierInput) (*VIPTier, error) {
	if s == nil || s.entClient == nil {
		return nil, infraerrors.InternalServer("VIP_UNAVAILABLE", "vip tiers are unavailable")
	}
	candidate := applyVIPTierInput(VIPTier{RateMultiplier: 1, Concurrency: 5, GraceDays: 60, Enabled: true}, in)
	if err := validateVIPTierFields(candidate); err != nil {
		return nil, err
	}
	existing, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateVIPLadder(append(existing, candidate)); err != nil {
		return nil, err
	}

	row, err := s.entClient.VIPTier.Create().
		SetLevel(candidate.Level).
		SetName(candidate.Name).
		SetMinSpendUsd(candidate.MinSpendUSD).
		SetRateMultiplier(candidate.RateMultiplier).
		SetConcurrency(candidate.Concurrency).
		SetRpmLimit(candidate.RPMLimit).
		SetUnlimitedConcurrency(candidate.UnlimitedConcurrency).
		SetUnlimitedRpm(candidate.UnlimitedRPM).
		SetGraceDays(candidate.GraceDays).
		SetBadgeColor(candidate.BadgeColor).
		SetEnabled(candidate.Enabled).
		Save(ctx)
	if err != nil {
		if dbent.IsConstraintError(err) {
			return nil, infraerrors.Conflict("LEVEL_TAKEN", "another tier already uses this level")
		}
		return nil, fmt.Errorf("create vip tier: %w", err)
	}
	return vipTierFromEnt(row), nil
}

// Update edits a tier. Existing members are not re-graded: raising a threshold
// must not demote the customers who already met the old one.
func (s *VIPTierService) Update(ctx context.Context, id int64, in VIPTierInput) (*VIPTier, error) {
	if s == nil || s.entClient == nil {
		return nil, infraerrors.InternalServer("VIP_UNAVAILABLE", "vip tiers are unavailable")
	}
	current, err := s.entClient.VIPTier.Get(ctx, id)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, infraerrors.NotFound("NOT_FOUND", "vip tier not found")
		}
		return nil, fmt.Errorf("get vip tier: %w", err)
	}
	candidate := applyVIPTierInput(*vipTierFromEnt(current), in)
	if err := validateVIPTierFields(candidate); err != nil {
		return nil, err
	}
	existing, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	others := make([]VIPTier, 0, len(existing))
	for _, tier := range existing {
		if tier.ID != id {
			others = append(others, tier)
		}
	}
	if err := validateVIPLadder(append(others, candidate)); err != nil {
		return nil, err
	}

	row, err := s.entClient.VIPTier.UpdateOneID(id).
		SetLevel(candidate.Level).
		SetName(candidate.Name).
		SetMinSpendUsd(candidate.MinSpendUSD).
		SetRateMultiplier(candidate.RateMultiplier).
		SetConcurrency(candidate.Concurrency).
		SetRpmLimit(candidate.RPMLimit).
		SetUnlimitedConcurrency(candidate.UnlimitedConcurrency).
		SetUnlimitedRpm(candidate.UnlimitedRPM).
		SetGraceDays(candidate.GraceDays).
		SetBadgeColor(candidate.BadgeColor).
		SetEnabled(candidate.Enabled).
		Save(ctx)
	if err != nil {
		if dbent.IsConstraintError(err) {
			return nil, infraerrors.Conflict("LEVEL_TAKEN", "another tier already uses this level")
		}
		return nil, fmt.Errorf("update vip tier: %w", err)
	}
	s.invalidateTierMembers(ctx, id)
	return vipTierFromEnt(row), nil
}

// Delete removes a tier and unranks whoever held it.
//
// Leaving the dangling id on those users would read as no tier anyway, but it
// would also leave them stuck: the grader only ever raises a tier, so nothing
// would give them a real one back.
func (s *VIPTierService) Delete(ctx context.Context, id int64) error {
	if s == nil || s.entClient == nil {
		return infraerrors.InternalServer("VIP_UNAVAILABLE", "vip tiers are unavailable")
	}
	members, err := s.entClient.User.Query().Where(dbuser.VipTierIDEQ(id)).IDs(ctx)
	if err != nil {
		return fmt.Errorf("list vip tier members: %w", err)
	}
	if len(members) > 0 {
		if _, err := s.entClient.User.Update().
			Where(dbuser.IDIn(members...)).
			ClearVipTierID().
			ClearVipExpiresAt().
			Save(ctx); err != nil {
			return fmt.Errorf("unrank vip tier members: %w", err)
		}
	}
	if err := s.entClient.VIPTier.DeleteOneID(id).Exec(ctx); err != nil {
		if dbent.IsNotFound(err) {
			return infraerrors.NotFound("NOT_FOUND", "vip tier not found")
		}
		return fmt.Errorf("delete vip tier: %w", err)
	}
	s.invalidateUsers(ctx, members)
	return nil
}

// SetUserTier pins or clears a user's tier by hand.
//
// A pinned tier is locked, which takes it out of both the grader's and the
// expiry sweep's reach â€” that is the point of the endpoint. Clearing unlocks
// so the user rejoins normal grading on their next order.
func (s *VIPTierService) SetUserTier(ctx context.Context, userID int64, tierID *int64) error {
	if s == nil || s.entClient == nil {
		return infraerrors.InternalServer("VIP_UNAVAILABLE", "vip tiers are unavailable")
	}
	update := s.entClient.User.UpdateOneID(userID)
	if tierID == nil {
		update = update.ClearVipTierID().ClearVipExpiresAt().SetVipTierLocked(false)
	} else {
		if _, err := s.entClient.VIPTier.Get(ctx, *tierID); err != nil {
			if dbent.IsNotFound(err) {
				return infraerrors.NotFound("NOT_FOUND", "vip tier not found")
			}
			return fmt.Errorf("get vip tier: %w", err)
		}
		update = update.SetVipTierID(*tierID).ClearVipExpiresAt().SetVipTierLocked(true)
	}
	if err := update.Exec(ctx); err != nil {
		if dbent.IsNotFound(err) {
			return infraerrors.NotFound("NOT_FOUND", "user not found")
		}
		return fmt.Errorf("set user vip tier: %w", err)
	}
	s.invalidateUsers(ctx, []int64{userID})
	return nil
}

// GetUserStatus reports a user's standing and how far the next tier is.
func (s *VIPTierService) GetUserStatus(ctx context.Context, userID int64) (*VIPStatus, error) {
	if s == nil || s.entClient == nil {
		return nil, infraerrors.InternalServer("VIP_UNAVAILABLE", "vip tiers are unavailable")
	}
	u, err := s.entClient.User.Query().Where(dbuser.IDEQ(userID)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, infraerrors.NotFound("NOT_FOUND", "user not found")
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	tiers, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	status := &VIPStatus{
		QualifyingSpend: u.VipQualifyingSpend,
		TotalPaidUSD:    u.TotalPaidUsd,
		Locked:          u.VipTierLocked,
		ExpiresAt:       u.VipExpiresAt,
	}
	// Report the same way billing reads it: a lapsed tier is no tier, whatever
	// the row still says, so the page cannot advertise a discount that the
	// gateway is no longer applying.
	active := u.VipTierID != nil && (u.VipTierLocked || (u.VipExpiresAt != nil && u.VipExpiresAt.After(time.Now())))
	for i := range tiers {
		if active && tiers[i].ID == *u.VipTierID {
			tier := tiers[i]
			status.Tier = &tier
		}
	}
	for i := range tiers {
		if !tiers[i].Enabled {
			continue
		}
		if status.Tier != nil && tiers[i].Level <= status.Tier.Level {
			continue
		}
		if tiers[i].MinSpendUSD <= u.VipQualifyingSpend {
			continue
		}
		if status.NextTier == nil || tiers[i].Level < status.NextTier.Level {
			tier := tiers[i]
			status.NextTier = &tier
			status.SpendToNextTier = tier.MinSpendUSD - u.VipQualifyingSpend
		}
	}
	return status, nil
}

func (s *VIPTierService) invalidateTierMembers(ctx context.Context, tierID int64) {
	if s.invalidator == nil {
		return
	}
	members, err := s.entClient.User.Query().Where(dbuser.VipTierIDEQ(tierID)).IDs(ctx)
	if err != nil {
		return
	}
	s.invalidateUsers(ctx, members)
}

func (s *VIPTierService) invalidateUsers(ctx context.Context, ids []int64) {
	if s.invalidator == nil {
		return
	}
	for _, id := range ids {
		s.invalidator.InvalidateAuthCacheByUserID(ctx, id)
	}
}
