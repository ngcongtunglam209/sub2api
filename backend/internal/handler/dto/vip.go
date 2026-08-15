package dto

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// VIPTier is the wire shape of a configured tier.
type VIPTier struct {
	ID             int64     `json:"id"`
	Level          int       `json:"level"`
	Name           string    `json:"name"`
	MinSpendUSD    float64   `json:"min_spend_usd"`
	RateMultiplier float64   `json:"rate_multiplier"`
	Concurrency    int       `json:"concurrency"`
	GraceDays      int       `json:"grace_days"`
	BadgeColor     string    `json:"badge_color"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// VIPStatus is a user's own standing plus the distance to the next tier.
type VIPStatus struct {
	Tier            *VIPTier   `json:"tier"`
	NextTier        *VIPTier   `json:"next_tier"`
	QualifyingSpend float64    `json:"qualifying_spend"`
	TotalPaidUSD    float64    `json:"total_paid_usd"`
	SpendToNextTier float64    `json:"spend_to_next_tier"`
	ExpiresAt       *time.Time `json:"expires_at"`
	Locked          bool       `json:"locked"`
}

func VIPTierFromService(t *service.VIPTier) *VIPTier {
	if t == nil {
		return nil
	}
	return &VIPTier{
		ID:             t.ID,
		Level:          t.Level,
		Name:           t.Name,
		MinSpendUSD:    t.MinSpendUSD,
		RateMultiplier: t.RateMultiplier,
		Concurrency:    t.Concurrency,
		GraceDays:      t.GraceDays,
		BadgeColor:     t.BadgeColor,
		Enabled:        t.Enabled,
		CreatedAt:      t.CreatedAt,
		UpdatedAt:      t.UpdatedAt,
	}
}

func VIPStatusFromService(s *service.VIPStatus) *VIPStatus {
	if s == nil {
		return nil
	}
	return &VIPStatus{
		Tier:            VIPTierFromService(s.Tier),
		NextTier:        VIPTierFromService(s.NextTier),
		QualifyingSpend: s.QualifyingSpend,
		TotalPaidUSD:    s.TotalPaidUSD,
		SpendToNextTier: s.SpendToNextTier,
		ExpiresAt:       s.ExpiresAt,
		Locked:          s.Locked,
	}
}
