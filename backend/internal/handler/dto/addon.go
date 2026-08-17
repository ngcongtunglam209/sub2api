package dto

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// AddonOffer is one sellable resource, priced and bounded, alongside what the
// caller already holds of it.
//
// Offer and holding are one object rather than two lists the client has to
// join: "2 slots of your 20, expiring on the 3rd" is a single sentence in the
// UI, and splitting it invites a client that renders a cap without the amount
// counted against it.
type AddonOffer struct {
	Kind string `json:"kind"`
	// UnitPrice buys UnitAmount of this kind for one month.
	UnitPrice  float64 `json:"unit_price"`
	UnitAmount int     `json:"unit_amount"`
	// Cap is the most one user may hold in total, purchased amount included.
	Cap int `json:"cap"`
	// Held is what the caller holds right now; expired add-ons are already
	// excluded, so Cap-Held is exactly what they may still buy.
	Held      int        `json:"held"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// AddonCatalogue is the whole store page in one response.
type AddonCatalogue struct {
	Offers    []AddonOffer `json:"offers"`
	MinMonths int          `json:"min_months"`
	MaxMonths int          `json:"max_months"`
}

// AddonPurchaseResult is what one completed purchase did.
//
// Price is echoed back because the client computed its own estimate from the
// catalogue and the server's figure is the one that was charged; a silent
// disagreement between the two is how a support ticket starts.
type AddonPurchaseResult struct {
	Kind      string    `json:"kind"`
	Amount    int       `json:"amount"`
	Months    int       `json:"months"`
	Price     float64   `json:"price"`
	Held      int       `json:"held"`
	ExpiresAt time.Time `json:"expires_at"`
}

// AddonPricing is the admin-editable half of the catalogue.
type AddonPricing struct {
	ConcurrencyUnitPrice float64 `json:"concurrency_unit_price"`
	ConcurrencyMax       int     `json:"concurrency_max"`
	RPMUnitPrice         float64 `json:"rpm_unit_price"`
	RPMStep              int     `json:"rpm_step"`
	RPMMax               int     `json:"rpm_max"`
	MinMonths            int     `json:"min_months"`
	MaxMonths            int     `json:"max_months"`
}

func AddonCatalogueFromService(c *service.AddonCatalogue) *AddonCatalogue {
	if c == nil {
		return nil
	}
	offers := make([]AddonOffer, 0, len(service.AddonKinds()))
	for _, kind := range service.AddonKinds() {
		unitAmount := 1
		if kind == service.AddonKindRPM {
			unitAmount = c.Pricing.RPMStep
		}
		offers = append(offers, AddonOffer{
			Kind:       string(kind),
			UnitPrice:  c.Pricing.UnitPrice(kind),
			UnitAmount: unitAmount,
			Cap:        c.Pricing.Cap(kind),
			Held:       c.Holdings.Amount(kind),
			ExpiresAt:  c.Holdings.ExpiresAt(kind),
		})
	}
	return &AddonCatalogue{
		Offers:    offers,
		MinMonths: c.Pricing.MinMonths,
		MaxMonths: c.Pricing.MaxMonths,
	}
}

func AddonPurchaseResultFromService(p *service.AddonPurchase) *AddonPurchaseResult {
	if p == nil {
		return nil
	}
	return &AddonPurchaseResult{
		Kind:      string(p.Kind),
		Amount:    p.Amount,
		Months:    p.Months,
		Price:     p.Price,
		Held:      p.HeldAfter,
		ExpiresAt: p.ExpiresAt,
	}
}

func AddonPricingFromService(p service.AddonPricing) AddonPricing {
	return AddonPricing{
		ConcurrencyUnitPrice: p.ConcurrencyUnitPrice,
		ConcurrencyMax:       p.ConcurrencyMax,
		RPMUnitPrice:         p.RPMUnitPrice,
		RPMStep:              p.RPMStep,
		RPMMax:               p.RPMMax,
		MinMonths:            p.MinMonths,
		MaxMonths:            p.MaxMonths,
	}
}
