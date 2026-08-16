package service

import (
	"strconv"
	"strings"
)

// Display currency is what a panel *shows*, and it is deliberately not the same
// concept as the payment currency a gateway *collects*.
//
// Every amount in this system is stored in USD (see `payment_orders.amount`,
// `users.balance`, `vip_tiers.min_spend_usd`). A display currency never changes
// a stored number; it only decides how that number is rendered for the reader,
// and which unit the recharge screen quotes before the gateway takes over.
//
// The mapping is by UI locale rather than by account or IP, because the locale
// is the one signal the reader controls directly: a Vietnamese reader who
// switches to English is asking to see dollars, and honouring that is less
// surprising than pinning the currency to a profile field they cannot see.
const (
	// DisplayCurrencyUSD is the canonical unit — the rate is always exactly 1,
	// so a missing or broken FX feed degrades to "show the stored number".
	DisplayCurrencyUSD = "USD"
	DisplayCurrencyCNY = "CNY"
	DisplayCurrencyVND = "VND"
)

// SettingDisplayUSDToCNYRate holds units of CNY per 1 USD, for the `zh` locale's
// display currency.
//
// It has no VND-style twin key because VND display reuses
// SettingSubscriptionUSDToVNDRate on purpose: a Vietnamese reader must see the
// same number on the plan card that the SePay gateway will actually charge, and
// two independently-maintained rates guarantee those two numbers drift apart.
const SettingDisplayUSDToCNYRate = "DISPLAY_USD_TO_CNY_RATE"

// DisplayFXRates maps a display currency to how many of its units one USD buys.
// USD is always present at 1. A currency is omitted entirely when no usable rate
// is known — the frontend then falls back to rendering USD rather than inventing
// a conversion, so a stale or unconfigured feed can never quote a wrong price.
type DisplayFXRates map[string]float64

// BuildDisplayFXRates assembles the public rate table from the two stored rate
// settings. Non-positive rates are treated as "not configured", which is the
// same convention normalizeUSDToVNDRate already uses for the checkout path.
func BuildDisplayFXRates(usdToCNY, usdToVND float64) DisplayFXRates {
	rates := DisplayFXRates{DisplayCurrencyUSD: 1}
	if usdToCNY > 0 {
		rates[DisplayCurrencyCNY] = usdToCNY
	}
	if usdToVND > 0 {
		rates[DisplayCurrencyVND] = usdToVND
	}
	return rates
}

// parseDisplayRate reads a stored rate setting. Anything unparseable or
// non-positive collapses to 0, i.e. "no rate", rather than to an error: a
// corrupt row must not take down the public settings endpoint.
func parseDisplayRate(raw string) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || value <= 0 {
		return 0
	}
	return value
}

// crossRateFromVND converts two "VND per unit" board quotes into a direct
// USD→CNY rate. The bank publishes every currency against the dong, so the dong
// cancels: (VND per USD) / (VND per CNY) = CNY per USD.
//
// Returning 0 rather than an error keeps a missing CNY row from failing the VND
// sync that shares the same fetch.
func crossRateFromVND(vndPerUSD, vndPerCNY float64) float64 {
	if vndPerUSD <= 0 || vndPerCNY <= 0 {
		return 0
	}
	return vndPerUSD / vndPerCNY
}
