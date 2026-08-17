package dto

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ResellerPlan is the wire shape of a purchasable reseller tier.
type ResellerPlan struct {
	ID               int64     `json:"id"`
	Level            int       `json:"level"`
	Name             string    `json:"name"`
	Price            float64   `json:"price"`
	CreditRate       float64   `json:"credit_rate"`
	ConcurrencyBonus int       `json:"concurrency_bonus"`
	RPMLimit         int       `json:"rpm_limit"`
	MaxDomains       int       `json:"max_domains"`
	ValidityDays     int       `json:"validity_days"`
	AllowedGroupIDs  []int64   `json:"allowed_group_ids"`
	Enabled          bool      `json:"enabled"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// ResellerPlanAssignment is what a user currently holds.
//
// Active is computed here rather than left to the client: an expired plan is
// still returned so the holder can see they had one, and "expires_at is in the
// past" is not a judgement every caller should have to make for itself.
type ResellerPlanAssignment struct {
	Plan      *ResellerPlan `json:"plan"`
	ExpiresAt time.Time     `json:"expires_at"`
	Active    bool          `json:"active"`
}

// ResellerDomain is a reseller-owned hostname pointed at this deployment.
type ResellerDomain struct {
	ID        int64     `json:"id"`
	Domain    string    `json:"domain"`
	UserID    int64     `json:"user_id"`
	Status    string    `json:"status"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ResellerCode is one code in a reseller's own listing.
//
// The code string is included: the reseller minted it against their own
// balance and has to be able to read it back to sell it. This shape is only
// ever served from a query already scoped to the caller's created_by.
type ResellerCode struct {
	ID        int64      `json:"id"`
	Code      string     `json:"code"`
	Type      string     `json:"type"`
	Value     float64    `json:"value"`
	Status    string     `json:"status"`
	GroupID   *int64     `json:"group_id"`
	Notes     string     `json:"notes"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at"`
}

func ResellerPlanFromService(p *service.ResellerPlan) *ResellerPlan {
	if p == nil {
		return nil
	}
	ids := p.AllowedGroupIDs
	if ids == nil {
		// Never null on the wire: a client checking `length === 0` should not
		// have to special-case a missing whitelist, which means "unrestricted".
		ids = []int64{}
	}
	return &ResellerPlan{
		ID:               p.ID,
		Level:            p.Level,
		Name:             p.Name,
		Price:            p.Price,
		CreditRate:       p.CreditRate,
		ConcurrencyBonus: p.ConcurrencyBonus,
		RPMLimit:         p.RPMLimit,
		MaxDomains:       p.MaxDomains,
		ValidityDays:     p.ValidityDays,
		AllowedGroupIDs:  ids,
		Enabled:          p.Enabled,
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
	}
}

// ResellerPlanAssignmentFromService returns nil for a user holding no plan, so
// the endpoint answers `null` rather than an assignment with a zero plan in it.
func ResellerPlanAssignmentFromService(a *service.ResellerPlanAssignment) *ResellerPlanAssignment {
	if a == nil || a.Plan == nil {
		return nil
	}
	return &ResellerPlanAssignment{
		Plan:      ResellerPlanFromService(a.Plan),
		ExpiresAt: a.ExpiresAt,
		Active:    a.Active(time.Now()),
	}
}

func ResellerDomainFromService(d *service.ResellerDomain) *ResellerDomain {
	if d == nil {
		return nil
	}
	return &ResellerDomain{
		ID:        d.ID,
		Domain:    d.Domain,
		UserID:    d.UserID,
		Status:    d.Status,
		Notes:     d.Notes,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}

func ResellerCodeFromService(c *service.RedeemCode) *ResellerCode {
	if c == nil {
		return nil
	}
	return &ResellerCode{
		ID:        c.ID,
		Code:      c.Code,
		Type:      c.Type,
		Value:     c.Value,
		Status:    c.Status,
		GroupID:   c.GroupID,
		Notes:     c.Notes,
		CreatedAt: c.CreatedAt,
		ExpiresAt: c.ExpiresAt,
		UsedAt:    c.UsedAt,
	}
}
