package service

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/ent/viptier"
)

// selectVIPTier picks the highest tier a spend figure has earned.
//
// tiers must be the enabled tiers; the caller is expected to have filtered
// disabled ones out, because turning a tier off means "no one new enters it"
// rather than "kick everyone out of it" — users already in it keep it until
// their expiry.
func selectVIPTier(tiers []*dbent.VIPTier, qualifyingSpend float64) *dbent.VIPTier {
	var best *dbent.VIPTier
	for _, tier := range tiers {
		if tier == nil || tier.MinSpendUsd > qualifyingSpend {
			continue
		}
		if best == nil || tier.MinSpendUsd > best.MinSpendUsd {
			best = tier
		}
	}
	return best
}

// vipTierExpiry returns when a tier granted now stops being valid.
func vipTierExpiry(tier *dbent.VIPTier, now time.Time) time.Time {
	days := tier.GraceDays
	if days <= 0 {
		days = 60
	}
	return now.AddDate(0, 0, days)
}

// applyVIPTierForUser re-grades a user after their spend changed.
//
// Called from the fulfillment transaction so the new spend and the tier it
// buys land together — a user who pays and then sees the old tier until some
// later job runs will open a ticket, and rightly so.
//
// Every completed order pushes the expiry out by the tier's grace days, which
// is what makes the tier "lapses if you stop spending" rather than "lapses on
// a fixed date": a customer who keeps buying never notices it exists.
func (s *PaymentService) applyVIPTierForUser(ctx context.Context, userID int64) error {
	if s == nil || s.entClient == nil || userID <= 0 {
		return nil
	}
	client := s.entClient
	if tx := dbent.TxFromContext(ctx); tx != nil {
		client = tx.Client()
	}

	u, err := client.User.Query().Where(dbuser.IDEQ(userID)).Only(ctx)
	if err != nil {
		return fmt.Errorf("load user for vip grading: %w", err)
	}
	// An admin-locked tier is a manual decision — a support gesture, a
	// negotiated contract — and must not be undone by the grader.
	if u.VipTierLocked {
		return nil
	}

	tiers, err := client.VIPTier.Query().Where(viptier.EnabledEQ(true)).All(ctx)
	if err != nil {
		return fmt.Errorf("load vip tiers: %w", err)
	}
	tier := selectVIPTier(tiers, u.VipQualifyingSpend)
	if tier == nil {
		// Below the entry threshold: leave whatever tier they already hold
		// alone. It has its own expiry, and clearing it here would demote a
		// paying customer the moment their window reset.
		return nil
	}

	// Raising a threshold in the admin UI must not demote the customers who
	// already qualified under the old one: keep the higher tier they hold and
	// only refresh its expiry. They lose it by going quiet, not by an edit.
	grant := tier
	if u.VipTierID != nil && *u.VipTierID != tier.ID {
		if current, err := client.VIPTier.Get(ctx, *u.VipTierID); err == nil && current.Level > tier.Level {
			grant = current
		}
	}

	now := time.Now()
	update := client.User.UpdateOneID(userID).
		SetVipTierID(grant.ID).
		SetVipExpiresAt(vipTierExpiry(grant, now))
	if err := update.Exec(ctx); err != nil {
		return fmt.Errorf("apply vip tier: %w", err)
	}
	return nil
}
