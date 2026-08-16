package service

import (
	"math"
	"testing"
)

func TestBuildDisplayFXRates_AlwaysPinsUSDToOne(t *testing.T) {
	rates := BuildDisplayFXRates(0, 0)

	if got := rates[DisplayCurrencyUSD]; got != 1 {
		t.Fatalf("USD rate = %v, want 1", got)
	}
}

// A currency without a usable rate must be absent rather than present-and-zero:
// the panel treats "absent" as "render dollars", and a zero would otherwise be
// read as a real rate and price every plan at nothing.
func TestBuildDisplayFXRates_OmitsUnconfiguredCurrencies(t *testing.T) {
	cases := []struct {
		name           string
		usdToCNY       float64
		usdToVND       float64
		wantCNYPresent bool
		wantVNDPresent bool
	}{
		{name: "both off", usdToCNY: 0, usdToVND: 0},
		{name: "vnd only", usdToCNY: 0, usdToVND: 26000, wantVNDPresent: true},
		{name: "cny only", usdToCNY: 7.1, usdToVND: 0, wantCNYPresent: true},
		{name: "negative rates rejected", usdToCNY: -7.1, usdToVND: -26000},
		{name: "both on", usdToCNY: 7.1, usdToVND: 26000, wantCNYPresent: true, wantVNDPresent: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rates := BuildDisplayFXRates(tc.usdToCNY, tc.usdToVND)

			if _, ok := rates[DisplayCurrencyCNY]; ok != tc.wantCNYPresent {
				t.Errorf("CNY present = %v, want %v (rates=%v)", ok, tc.wantCNYPresent, rates)
			}
			if _, ok := rates[DisplayCurrencyVND]; ok != tc.wantVNDPresent {
				t.Errorf("VND present = %v, want %v (rates=%v)", ok, tc.wantVNDPresent, rates)
			}
		})
	}
}

func TestParseDisplayRate(t *testing.T) {
	cases := []struct {
		raw  string
		want float64
	}{
		{raw: "26320", want: 26320},
		{raw: "7.1234", want: 7.1234},
		{raw: "  7.1234  ", want: 7.1234},
		{raw: "", want: 0},
		{raw: "0", want: 0},
		{raw: "-5", want: 0},
		// A corrupted row must degrade to "no rate", not take down the public
		// settings endpoint that every unauthenticated page load depends on.
		{raw: "not-a-number", want: 0},
		{raw: "26,320", want: 0},
	}

	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			if got := parseDisplayRate(tc.raw); got != tc.want {
				t.Errorf("parseDisplayRate(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

// The bank quotes every currency against the dong, so dividing the dollar quote
// by the yuan quote cancels the dong and leaves CNY per USD.
func TestCrossRateFromVND(t *testing.T) {
	got := crossRateFromVND(26320, 3700)
	want := 26320.0 / 3700.0

	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("crossRateFromVND = %v, want %v", got, want)
	}
	if got < 6 || got > 8 {
		t.Fatalf("cross rate %v is outside any plausible USD/CNY band", got)
	}
}

func TestCrossRateFromVND_RejectsUnusableInputs(t *testing.T) {
	cases := []struct {
		name      string
		vndPerUSD float64
		vndPerCNY float64
	}{
		{name: "missing usd quote", vndPerUSD: 0, vndPerCNY: 3700},
		{name: "missing cny quote", vndPerUSD: 26320, vndPerCNY: 0},
		{name: "negative usd quote", vndPerUSD: -26320, vndPerCNY: 3700},
		{name: "both missing", vndPerUSD: 0, vndPerCNY: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := crossRateFromVND(tc.vndPerUSD, tc.vndPerCNY); got != 0 {
				t.Errorf("crossRateFromVND(%v, %v) = %v, want 0", tc.vndPerUSD, tc.vndPerCNY, got)
			}
		})
	}
}

// Margin is the operator's markup on top of the board rate. It must land on the
// yuan display rate as well as the dong one, or the two locales quote prices
// that disagree by the margin.
func TestCrossRateCarriesTheSameMarginAsTheVNDRate(t *testing.T) {
	const margin = 2.0

	vndPerUSD, vndPerCNY := 26320.0, 3700.0
	cross := applyVNDRateMargin(crossRateFromVND(vndPerUSD, vndPerCNY), margin)
	vnd := applyVNDRateMargin(vndPerUSD, margin)

	// Both rates are the raw board number widened by the same factor, so their
	// ratio is unchanged by the margin.
	gotRatio := vnd / cross
	if math.Abs(gotRatio-vndPerCNY) > 1e-6 {
		t.Fatalf("VND/CNY implied by margined rates = %v, want %v", gotRatio, vndPerCNY)
	}
}
