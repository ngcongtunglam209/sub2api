//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/branding"
)

func brandingSettingsRepo() *settingPublicRepoStub {
	return &settingPublicRepoStub{
		values: map[string]string{
			SettingKeySiteName:     "House Brand",
			SettingKeySiteLogo:     "https://cdn.house.example/logo.png",
			SettingKeySiteSubtitle: "House Gateway",
			// Iframe origins, the only settings the CSP cache reads.
			SettingKeyHomeContent:                 "https://home.house.example/welcome",
			SettingKeyPurchaseSubscriptionEnabled: "true",
			SettingKeyPurchaseSubscriptionURL:     "https://buy.house.example/plans",
		},
	}
}

// GetPublicSettings is the one choke point both consumers share, so overriding
// there is what makes GET /settings/public and the injected __APP_CONFIG__
// agree without either caller knowing the feature exists.
func TestGetPublicSettingsAppliesTheResolvedHostBranding(t *testing.T) {
	svc := NewSettingService(brandingSettingsRepo(), &config.Config{})

	ctx := branding.NewContext(context.Background(), branding.Host{
		DomainID:     7,
		SiteName:     "Reseller",
		SiteLogo:     "https://cdn.reseller.example/logo.svg",
		SiteSubtitle: "Reseller Gateway",
	})

	settings, err := svc.GetPublicSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, "Reseller", settings.SiteName)
	require.Equal(t, "https://cdn.reseller.example/logo.svg", settings.SiteLogo)
	require.Equal(t, "Reseller Gateway", settings.SiteSubtitle)
}

// A domain row with only one field set keeps the deployment's values for the
// rest. Empty means "unset", never "blank".
func TestGetPublicSettingsFallsBackPerField(t *testing.T) {
	svc := NewSettingService(brandingSettingsRepo(), &config.Config{})

	ctx := branding.NewContext(context.Background(), branding.Host{DomainID: 7, SiteName: "Reseller"})

	settings, err := svc.GetPublicSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, "Reseller", settings.SiteName)
	require.Equal(t, "https://cdn.house.example/logo.png", settings.SiteLogo, "an unset logo must fall back to the deployment's")
	require.Equal(t, "House Gateway", settings.SiteSubtitle)
}

// The canonical host, an unknown host, a health check and every background job
// reach here with no branding in context and must see today's settings exactly.
func TestGetPublicSettingsWithoutBrandingIsUnchanged(t *testing.T) {
	svc := NewSettingService(brandingSettingsRepo(), &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, "House Brand", settings.SiteName)
	require.Equal(t, "https://cdn.house.example/logo.png", settings.SiteLogo)
	require.Equal(t, "House Gateway", settings.SiteSubtitle)

	// A resolved host that overrides nothing is the same case.
	ctx := branding.NewContext(context.Background(), branding.Host{DomainID: 9})
	settings, err = svc.GetPublicSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, "House Brand", settings.SiteName)
}

// The router stores GetFrameSrcOrigins' result in one process-wide pointer,
// refreshed only when settings change. Anything host-dependent reaching it
// would apply one reseller's CSP to every other host until the next settings
// update — so this call must read the global settings, whatever is in context.
func TestGetFrameSrcOriginsIgnoresTheHostBranding(t *testing.T) {
	svc := NewSettingService(brandingSettingsRepo(), &config.Config{})

	global, err := svc.GetFrameSrcOrigins(context.Background())
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"https://home.house.example", "https://buy.house.example"}, global)

	ctx := branding.NewContext(context.Background(), branding.Host{
		DomainID:     7,
		SiteName:     "Reseller",
		SiteLogo:     "https://cdn.reseller.example/logo.svg",
		SiteSubtitle: "Reseller Gateway",
	})

	// Guard against the test passing because the context was inert: the same
	// context must genuinely override the branding.
	branded, err := svc.GetPublicSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, "Reseller", branded.SiteName)

	perHost, err := svc.GetFrameSrcOrigins(ctx)
	require.NoError(t, err)
	require.Equal(t, global, perHost)
}
