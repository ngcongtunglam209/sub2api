package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/branding"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

var brandingRows = []service.ActiveResellerDomain{
	{ID: 7, Domain: "api.brand.com", SiteName: "Brand", SiteLogo: "https://cdn.brand.com/logo.png"},
	{ID: 11, Domain: "plain.example.net"},
}

// resolvedBranding runs one request through the middleware and reports what the
// handler downstream would read out of the context.
func resolvedBranding(t *testing.T, cfg config.CustomDomainConfig, host, path string) branding.Host {
	t.Helper()
	gin.SetMode(gin.TestMode)

	svc := service.NewResellerDomainService(&guardStubRepo{rows: brandingRows}, cfg.CanonicalHosts)

	var seen branding.Host
	r := gin.New()
	r.Use(HostBranding(cfg, svc))
	r.GET("/*any", func(c *gin.Context) {
		seen = branding.FromContext(c.Request.Context())
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = host
	r.ServeHTTP(httptest.NewRecorder(), req)
	return seen
}

func enabledCustomDomain() config.CustomDomainConfig {
	return config.CustomDomainConfig{Enabled: true, CanonicalHosts: []string{"api.lamtung.dev"}}
}

// The point of the custom domain: the reseller's own hostname renders their
// name, and everything downstream reads it from one resolution.
func TestHostBrandingPutsTheResellersBrandingInContext(t *testing.T) {
	host := resolvedBranding(t, enabledCustomDomain(), "api.brand.com", "/")

	require.Equal(t, int64(7), host.DomainID)
	require.Equal(t, "Brand", host.SiteName)
	require.Equal(t, "https://cdn.brand.com/logo.png", host.SiteLogo)
	require.NotEqual(t, branding.DefaultCacheKey, host.CacheKey())
}

// Everything else renders exactly what it rendered before this middleware
// existed — including the health check and the TLS ask endpoint, which are not
// special-cased precisely because falling back is already the default.
func TestHostBrandingLeavesEverythingElseOnTheGlobalBranding(t *testing.T) {
	cfg := enabledCustomDomain()

	for _, tc := range []struct{ name, host, path string }{
		{name: "canonical host", host: "api.lamtung.dev", path: "/"},
		{name: "unknown host", host: "evil.example.com", path: "/"},
		{name: "registered but unbranded", host: "plain.example.net", path: "/"},
		{name: "health check", host: "localhost:8080", path: "/health"},
		{name: "tls ask endpoint", host: "api.brand.com", path: "/internal/tls-check"},
		{name: "no host at all", host: "", path: "/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host := resolvedBranding(t, cfg, tc.host, tc.path)
			require.False(t, host.HasOverride())
			require.Equal(t, branding.DefaultCacheKey, host.CacheKey())
		})
	}
}

// With custom domains off, no host but this deployment's own is served — so
// there is no second branding to resolve and no reason to touch the database.
func TestHostBrandingDisabledIsAPassThrough(t *testing.T) {
	host := resolvedBranding(t, config.CustomDomainConfig{Enabled: false}, "api.brand.com", "/")

	require.False(t, host.HasOverride())
	require.Equal(t, branding.DefaultCacheKey, host.CacheKey())
}
