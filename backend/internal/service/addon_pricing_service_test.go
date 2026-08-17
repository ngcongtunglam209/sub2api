package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// Unset keys must price a working store, not a free one: a fresh install that
// read zeroes would give the scarcest resource in the system away.
func TestAddonPricingFallsBackToDefaults(t *testing.T) {
	svc := NewAddonPricingService(newStubAddonSettingRepo())

	pricing, err := svc.Get(context.Background())
	require.NoError(t, err)

	require.InDelta(t, defaultAddonConcurrencyUnitPrice, pricing.ConcurrencyUnitPrice, 1e-9)
	require.Equal(t, defaultAddonConcurrencyMax, pricing.ConcurrencyMax)
	require.InDelta(t, defaultAddonRPMUnitPrice, pricing.RPMUnitPrice, 1e-9)
	require.Equal(t, defaultAddonRPMMax, pricing.RPMMax)
	require.Equal(t, rpmAddonStep, pricing.RPMStep)
	require.Equal(t, minAddonMonths, pricing.MinMonths)
	require.Equal(t, maxAddonMonths, pricing.MaxMonths)
}

// A corrupted or half-written key falls back to the default rather than to
// zero, for the same reason.
func TestAddonPricingIgnoresUnparseableValues(t *testing.T) {
	settings := newStubAddonSettingRepo()
	settings.values[SettingAddonConcurrencyUnitPrice] = "free please"
	settings.values[SettingAddonRPMMax] = ""

	pricing, err := NewAddonPricingService(settings).Get(context.Background())
	require.NoError(t, err)

	require.InDelta(t, defaultAddonConcurrencyUnitPrice, pricing.ConcurrencyUnitPrice, 1e-9)
	require.Equal(t, defaultAddonRPMMax, pricing.RPMMax)
}

func TestAddonPricingUpdateBounds(t *testing.T) {
	for _, tc := range []struct {
		name   string
		in     UpdateAddonPricingRequest
		wantOK bool
	}{
		{name: "no-op", in: UpdateAddonPricingRequest{}, wantOK: true},
		{name: "raise the slot price", in: UpdateAddonPricingRequest{ConcurrencyUnitPrice: float64Ptr(3.5)}, wantOK: true},
		{name: "a free slot is a giveaway", in: UpdateAddonPricingRequest{ConcurrencyUnitPrice: float64Ptr(0)}},
		{name: "negative price", in: UpdateAddonPricingRequest{ConcurrencyUnitPrice: float64Ptr(-1)}},
		{name: "negative rpm price", in: UpdateAddonPricingRequest{RPMUnitPrice: float64Ptr(-0.01)}},
		{name: "lower the cap", in: UpdateAddonPricingRequest{ConcurrencyMax: intPtr(8)}, wantOK: true},
		{name: "zero cap takes it off sale", in: UpdateAddonPricingRequest{ConcurrencyMax: intPtr(0)}, wantOK: true},
		{name: "negative cap", in: UpdateAddonPricingRequest{ConcurrencyMax: intPtr(-1)}},
		{name: "rpm cap on a block boundary", in: UpdateAddonPricingRequest{RPMMax: intPtr(300)}, wantOK: true},
		// A cap that is not a whole number of blocks can never be reached: the
		// purchase that would fill it is refused for being a partial block.
		{name: "rpm cap off a block boundary", in: UpdateAddonPricingRequest{RPMMax: intPtr(305)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := newStubAddonSettingRepo()
			svc := NewAddonPricingService(settings)

			_, err := svc.Update(context.Background(), tc.in)
			if !tc.wantOK {
				require.Error(t, err)
				require.Empty(t, settings.values, "a rejected edit must never be written")
				return
			}
			require.NoError(t, err)
		})
	}
}

// Edits are a patch: raising one price must not zero the caps.
func TestAddonPricingUpdateLeavesOmittedFieldsAlone(t *testing.T) {
	settings := newStubAddonSettingRepo()
	svc := NewAddonPricingService(settings)
	ctx := context.Background()

	_, err := svc.Update(ctx, UpdateAddonPricingRequest{ConcurrencyMax: intPtr(6)})
	require.NoError(t, err)

	pricing, err := svc.Update(ctx, UpdateAddonPricingRequest{RPMUnitPrice: float64Ptr(2)})
	require.NoError(t, err)

	require.Equal(t, 6, pricing.ConcurrencyMax)
	require.InDelta(t, 2, pricing.RPMUnitPrice, 1e-9)
	require.InDelta(t, defaultAddonConcurrencyUnitPrice, pricing.ConcurrencyUnitPrice, 1e-9)
	require.Equal(t, defaultAddonRPMMax, pricing.RPMMax)
}
