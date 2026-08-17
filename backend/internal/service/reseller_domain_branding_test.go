//go:build unit

package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/pkg/branding"
)

func brandedRepo() *stubResellerDomainRepo {
	return &stubResellerDomainRepo{
		rows: []ActiveResellerDomain{
			{ID: 7, Domain: "api.brand.com", SiteName: "Brand", SiteLogo: "https://cdn.brand.com/logo.png", SiteSubtitle: "Brand Gateway"},
			{ID: 9, Domain: "gw.other.io", SiteName: "Other"},
			{ID: 11, Domain: "plain.example.net"},
		},
	}
}

// The whole point of the custom domain is that the reseller's own name is what
// their customers see.
func TestResolveHostBrandingReturnsTheDomainsOwnBranding(t *testing.T) {
	svc := NewResellerDomainService(brandedRepo(), []string{"panel.us.example"})

	host := svc.ResolveHostBranding(context.Background(), "api.brand.com")
	require.Equal(t, int64(7), host.DomainID)
	require.Equal(t, "Brand", host.SiteName)
	require.Equal(t, "https://cdn.brand.com/logo.png", host.SiteLogo)
	require.Equal(t, "Brand Gateway", host.SiteSubtitle)

	// Host is client-controlled: every spelling of the same name has to resolve
	// to the same branding, or a reseller's own customers see the wrong panel
	// depending on whether their client sent a port.
	require.Equal(t, host, svc.ResolveHostBranding(context.Background(), "API.Brand.com:443"))
}

// Each field falls back on its own: a reseller who only wanted their name on
// the tab keeps our logo rather than losing it.
func TestResolveHostBrandingLeavesUnsetFieldsEmpty(t *testing.T) {
	svc := NewResellerDomainService(brandedRepo(), nil)

	host := svc.ResolveHostBranding(context.Background(), "gw.other.io")
	require.Equal(t, "Other", host.SiteName)
	require.Empty(t, host.SiteLogo, "an unset logo must fall back, not blank the page")
	require.Empty(t, host.SiteSubtitle)
}

// Everything that is not a configured reseller brand renders exactly what it
// rendered before this feature existed — and shares one cache identity.
func TestResolveHostBrandingFallsBackToTheDeploymentsOwn(t *testing.T) {
	svc := NewResellerDomainService(brandedRepo(), []string{"panel.us.example"})

	for _, name := range []string{
		"panel.us.example",  // canonical host
		"unknown.example",   // never registered
		"plain.example.net", // registered, no branding configured
		"",                  // no Host header at all
	} {
		t.Run(name, func(t *testing.T) {
			host := svc.ResolveHostBranding(context.Background(), name)
			require.False(t, host.HasOverride())
			require.Equal(t, branding.DefaultCacheKey, host.CacheKey())
		})
	}
}

// Two resellers must never share a cache identity — that collision is the leak
// this key exists to prevent.
func TestResolveHostBrandingKeysAreDistinctPerDomain(t *testing.T) {
	svc := NewResellerDomainService(brandedRepo(), nil)

	a := svc.ResolveHostBranding(context.Background(), "api.brand.com").CacheKey()
	b := svc.ResolveHostBranding(context.Background(), "gw.other.io").CacheKey()

	require.NotEqual(t, a, b)
	require.NotEqual(t, branding.DefaultCacheKey, a)
	require.NotEqual(t, branding.DefaultCacheKey, b)

	// The key must not be derived from the Host header — that is the value an
	// attacker picks — so no spelling of the hostname may appear in it.
	require.NotContains(t, a, "brand.com")
}

// One snapshot answers both questions. A second query on the request path is
// the thing this design exists to avoid.
func TestResolveHostBrandingSharesTheAllowlistSnapshot(t *testing.T) {
	repo := brandedRepo()
	svc := NewResellerDomainService(repo, nil)

	for i := 0; i < 50; i++ {
		require.True(t, svc.IsAllowedHost(context.Background(), "api.brand.com"))
		require.Equal(t, "Brand", svc.ResolveHostBranding(context.Background(), "api.brand.com").SiteName)
		svc.ResolveHostBranding(context.Background(), "flood-the-cache.example")
	}

	require.Equal(t, 1, repo.callCount())
}

// Branding is cosmetic. Unlike the allow decision — which gates certificate
// issuance and fails closed — a database blip must not blank the panel.
func TestResolveHostBrandingDegradesToTheGlobalBranding(t *testing.T) {
	repo := &stubResellerDomainRepo{err: errors.New("database unavailable")}
	svc := NewResellerDomainService(repo, nil)

	host := svc.ResolveHostBranding(context.Background(), "api.brand.com")
	require.False(t, host.HasOverride())
	require.Equal(t, branding.DefaultCacheKey, host.CacheKey())
}

func TestUpdateBrandingWritesOnlyTheAddressedFields(t *testing.T) {
	repo := brandedRepo()
	svc := NewResellerDomainService(repo, nil)

	name := "  Renamed  "
	require.NoError(t, svc.UpdateBranding(context.Background(), 7, ResellerDomainBrandingUpdate{SiteName: &name}))

	calls, id, update := repo.brandingState()
	require.Equal(t, 1, calls)
	require.Equal(t, int64(7), id)
	require.NotNil(t, update.SiteName)
	require.Equal(t, "Renamed", *update.SiteName, "surrounding space is not part of a brand name")
	require.Nil(t, update.SiteLogo, "an unaddressed field must not be touched")
	require.Nil(t, update.SiteSubtitle)
}

// Empty means "clear the override", which is the only way back to the
// deployment's own branding — not "render a blank panel".
func TestUpdateBrandingTreatsEmptyAsClear(t *testing.T) {
	repo := brandedRepo()
	svc := NewResellerDomainService(repo, nil)

	empty := ""
	require.NoError(t, svc.UpdateBranding(context.Background(), 7, ResellerDomainBrandingUpdate{SiteLogo: &empty}))

	_, _, update := repo.brandingState()
	require.NotNil(t, update.SiteLogo)
	require.Equal(t, "", *update.SiteLogo)
}

// A branding edit shares the snapshot with the allow decision, so it has to
// expire it — otherwise the operator's change appears to do nothing for a
// minute and they "fix" it by editing again.
func TestUpdateBrandingInvalidatesTheSnapshot(t *testing.T) {
	repo := brandedRepo()
	svc := NewResellerDomainService(repo, nil)

	require.Equal(t, "Brand", svc.ResolveHostBranding(context.Background(), "api.brand.com").SiteName)
	require.Equal(t, 1, repo.callCount())

	name := "Renamed"
	require.NoError(t, svc.UpdateBranding(context.Background(), 7, ResellerDomainBrandingUpdate{SiteName: &name}))

	svc.ResolveHostBranding(context.Background(), "api.brand.com")
	require.Equal(t, 2, repo.callCount())
}

func TestUpdateBrandingRejectsValuesThatWouldNotRender(t *testing.T) {
	for _, tc := range []struct {
		name   string
		update func() ResellerDomainBrandingUpdate
	}{
		{
			name: "site_name over the column width",
			update: func() ResellerDomainBrandingUpdate {
				v := strings.Repeat("x", maxResellerSiteNameLen+1)
				return ResellerDomainBrandingUpdate{SiteName: &v}
			},
		},
		{
			name: "site_subtitle over the column width",
			update: func() ResellerDomainBrandingUpdate {
				v := strings.Repeat("x", maxResellerSiteSubtitleLen+1)
				return ResellerDomainBrandingUpdate{SiteSubtitle: &v}
			},
		},
		{
			name: "site_logo over the data URI allowance",
			update: func() ResellerDomainBrandingUpdate {
				v := strings.Repeat("x", maxResellerSiteLogoLen+1)
				return ResellerDomainBrandingUpdate{SiteLogo: &v}
			},
		},
		{
			// Control characters survive HTML escaping intact and never appear
			// in a name anyone meant to type.
			name: "control characters in the name",
			update: func() ResellerDomainBrandingUpdate {
				v := "Bra\x00nd"
				return ResellerDomainBrandingUpdate{SiteName: &v}
			},
		},
		{
			name: "newline smuggled into the logo URL",
			update: func() ResellerDomainBrandingUpdate {
				v := "https://cdn.brand.com/logo.png\n<script>"
				return ResellerDomainBrandingUpdate{SiteLogo: &v}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := brandedRepo()
			svc := NewResellerDomainService(repo, nil)

			require.Error(t, svc.UpdateBranding(context.Background(), 7, tc.update()))

			calls, _, _ := repo.brandingState()
			require.Zero(t, calls, "a rejected value must never reach the database")
		})
	}
}

// An empty PATCH is a no-op, not a write of three empty strings.
func TestUpdateBrandingIgnoresAnEmptyUpdate(t *testing.T) {
	repo := brandedRepo()
	svc := NewResellerDomainService(repo, nil)

	require.NoError(t, svc.UpdateBranding(context.Background(), 7, ResellerDomainBrandingUpdate{}))

	calls, _, _ := repo.brandingState()
	require.Zero(t, calls)
}

// The rendered HTML is cached per branding identity and has no TTL, so a
// branding edit that only expired the domain snapshot would leave the
// reseller's first paint on the old name until the next global settings change
// or a restart — the point at which an operator decides the feature is broken.
func TestInvalidateNotifiesTheRenderedHTMLCache(t *testing.T) {
	repo := brandedRepo()
	svc := NewResellerDomainService(repo, nil)

	invalidations := 0
	svc.SetOnInvalidateCallback(func() { invalidations++ })

	name := "Renamed"
	require.NoError(t, svc.UpdateBranding(context.Background(), 7, ResellerDomainBrandingUpdate{SiteName: &name}))
	require.Equal(t, 1, invalidations)

	require.NoError(t, svc.SetStatus(context.Background(), 7, "disabled"))
	require.Equal(t, 2, invalidations)
}

// Unset, nothing is notified and behaviour is exactly what it was.
func TestInvalidateWithoutACallbackIsSafe(t *testing.T) {
	svc := NewResellerDomainService(brandedRepo(), nil)
	require.NotPanics(t, svc.Invalidate)
}
