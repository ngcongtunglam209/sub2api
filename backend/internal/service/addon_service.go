package service

import (
	"context"
	"fmt"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// AddonService is the self-service store: a user spends their own balance on
// concurrency or RPM.
//
// No payment gateway is involved and none should be. The money arrived through
// the existing top-up flow; a purchase here is an internal ledger move — debit
// the balance, apply the thing — which is why every one of them fits in a
// single transaction and none of them has an order, a callback, or a
// reconciliation job.
type AddonService struct {
	repo    UserAddonRepository
	pricing *AddonPricingService

	// invalidator flushes the auth snapshot so a purchase takes effect now
	// rather than at the end of the cache TTL. Optional: without it the user
	// still gets what they bought, just a minute later.
	invalidator APIKeyAuthCacheInvalidator
}

func NewAddonService(
	repo UserAddonRepository,
	pricing *AddonPricingService,
) *AddonService {
	return &AddonService{repo: repo, pricing: pricing}
}

// SetAddonResolver injects the add-on lookup the auth snapshot uses for the
// purchased concurrency and RPM.
//
// Set after construction rather than taken as a constructor argument: the
// add-on service is built later in the dependency graph than the API key
// service. Left unset, purchased add-ons do not apply — the behaviour before
// the store existed.
func (s *APIKeyService) SetAddonResolver(resolver AddonResolver) {
	if s != nil {
		s.addonResolver = resolver
	}
}

// SetAuthCacheInvalidator injects the optional snapshot invalidator.
func (s *AddonService) SetAuthCacheInvalidator(invalidator APIKeyAuthCacheInvalidator) {
	if s != nil {
		s.invalidator = invalidator
	}
}

// AddonCatalogue is what the store page needs in one call: the prices and caps
// on offer, and what the caller already holds against them.
type AddonCatalogue struct {
	Pricing  AddonPricing
	Holdings AddonHoldings
}

// Catalogue answers GET /api/v1/addons.
func (s *AddonService) Catalogue(ctx context.Context, userID int64) (*AddonCatalogue, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.InternalServer("ADDON_STORE_UNAVAILABLE", "the add-on store is not configured")
	}
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "a user is required")
	}

	pricing, err := s.pricing.Get(ctx)
	if err != nil {
		return nil, err
	}
	holdings, err := s.ResolveActiveAddons(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &AddonCatalogue{Pricing: pricing, Holdings: holdings}, nil
}

// ResolveActiveAddons reports what a user holds right now, expired rows
// already discarded.
//
// Expiry is decided here, on read, rather than by the sweep. The sweep is
// housekeeping: if it stalls, gets disabled, or loses its leader lock, nobody
// should keep the concurrency they stopped paying for.
func (s *AddonService) ResolveActiveAddons(ctx context.Context, userID int64) (AddonHoldings, error) {
	if s == nil || s.repo == nil || userID <= 0 {
		return AddonHoldings{}, nil
	}

	rows, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return AddonHoldings{}, err
	}

	now := time.Now()
	var holdings AddonHoldings
	for _, row := range rows {
		if !row.Active(now) {
			continue
		}
		expires := row.ExpiresAt
		switch row.Kind {
		case AddonKindConcurrency:
			holdings.Concurrency += row.Amount
			holdings.ConcurrencyExpiresAt = &expires
		case AddonKindRPM:
			holdings.RPM += row.Amount
			holdings.RPMExpiresAt = &expires
		}
	}
	return holdings, nil
}

// Purchase buys concurrency or RPM with the caller's own balance.
//
// The order of operations is the whole point:
//
//   - the bounds and the price are settled before anything is written, so a
//     rejected order never touches the database;
//   - the cap is checked against what the user *already holds*, not just the
//     amount asked for, so twenty purchases of one slot cannot walk past a cap
//     of twenty;
//   - the debit and the write share one transaction, because debit-then-crash
//     takes money for nothing and write-then-crash gives it away;
//   - the balance check lives inside the UPDATE rather than in a SELECT before
//     it, because two concurrent purchases would each pass a pre-check and
//     together overdraw.
func (s *AddonService) Purchase(ctx context.Context, userID int64, kind AddonKind, amount, months int) (*AddonPurchase, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.InternalServer("ADDON_STORE_UNAVAILABLE", "the add-on store is not configured")
	}
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "a user is required")
	}
	if !kind.Valid() {
		return nil, infraerrors.BadRequest("INVALID_ADDON_KIND", "unknown add-on kind")
	}
	if err := validateAddonMonths(months); err != nil {
		return nil, err
	}
	units, err := addonPricedUnits(kind, amount)
	if err != nil {
		return nil, err
	}

	pricing, err := s.pricing.Get(ctx)
	if err != nil {
		return nil, err
	}
	unitPrice := pricing.UnitPrice(kind)
	if unitPrice <= 0 {
		return nil, infraerrors.BadRequest("ADDON_NOT_FOR_SALE", "this add-on is not currently for sale")
	}
	limit := pricing.Cap(kind)
	if limit <= 0 {
		return nil, infraerrors.BadRequest("ADDON_NOT_FOR_SALE", "this add-on is not currently for sale")
	}
	// The requested amount alone cannot exceed the cap either — cheap to check
	// here, and it keeps the in-transaction check from being the only thing
	// standing between a fat-fingered 10000 and a price to match.
	if amount > limit {
		return nil, addonCapExceeded(kind, limit)
	}

	price := calculateAddonPrice(units, unitPrice, months)
	if price <= 0 {
		return nil, infraerrors.BadRequest("INVALID_ADDON_PRICE", "this order prices to nothing")
	}

	result := &AddonPurchase{Kind: kind, Amount: amount, Months: months, Price: price}

	err = s.repo.RunInTx(ctx, func(txCtx context.Context) error {
		current, err := s.repo.LockByUserKind(txCtx, userID, kind)
		if err != nil {
			return err
		}

		now := time.Now()
		held := 0
		if current.Active(now) {
			held = current.Amount
		}
		if held+amount > limit {
			return addonCapExceeded(kind, limit)
		}

		if err := s.repo.DebitBalanceGuarded(txCtx, userID, price); err != nil {
			return err
		}

		expiresAt := extendAddonExpiry(current, now, months)
		if _, err := s.repo.Upsert(txCtx, userID, kind, held+amount, expiresAt); err != nil {
			return err
		}

		result.HeldAfter = held + amount
		result.ExpiresAt = expiresAt
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.invalidateAuthCache(ctx, userID)
	return result, nil
}

// invalidateAuthCache drops the caller's cached auth snapshots so the limit
// they just bought applies to their next request.
//
// Best effort by design: the snapshot expires on its own, and failing a
// purchase that has already been committed would be strictly worse than a
// minute of stale concurrency.
func (s *AddonService) invalidateAuthCache(ctx context.Context, userID int64) {
	if s == nil || s.invalidator == nil || userID <= 0 {
		return
	}
	s.invalidator.InvalidateAuthCacheByUserID(ctx, userID)
}

func addonCapExceeded(kind AddonKind, limit int) error {
	return infraerrors.BadRequest("ADDON_CAP_EXCEEDED",
		fmt.Sprintf("you may hold at most %d %s in total", limit, kind))
}
