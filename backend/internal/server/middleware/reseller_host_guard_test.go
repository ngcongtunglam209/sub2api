package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type guardStubRepo struct {
	// domains seeds hostnames with no branding override; rows seeds whole
	// snapshot rows when the test cares about the branding they carry.
	domains []string
	rows    []service.ActiveResellerDomain
}

func (s *guardStubRepo) ListActiveDomains(context.Context) ([]service.ActiveResellerDomain, error) {
	out := append([]service.ActiveResellerDomain(nil), s.rows...)
	for i, d := range s.domains {
		out = append(out, service.ActiveResellerDomain{ID: int64(i + 1), Domain: d})
	}
	return out, nil
}

func (s *guardStubRepo) UpdateBranding(context.Context, int64, service.ResellerDomainBrandingUpdate) error {
	return nil
}

func (s *guardStubRepo) Create(context.Context, *service.ResellerDomain) (*service.ResellerDomain, error) {
	return nil, nil
}

func (s *guardStubRepo) ListByUser(context.Context, int64) ([]*service.ResellerDomain, error) {
	return nil, nil
}

func (s *guardStubRepo) List(context.Context) ([]*service.ResellerDomain, error) { return nil, nil }
func (s *guardStubRepo) SetStatus(context.Context, int64, string) error          { return nil }
func (s *guardStubRepo) Delete(context.Context, int64) error                     { return nil }

func guardResponse(t *testing.T, cfg config.CustomDomainConfig, domains []string, host, path string) int {
	t.Helper()
	gin.SetMode(gin.TestMode)

	svc := service.NewResellerDomainService(&guardStubRepo{domains: domains}, cfg.CanonicalHosts)
	r := gin.New()
	r.Use(ResellerHostGuard(cfg, svc))
	r.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/internal/tls-check", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/api/v1/ping", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = host
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

// Deploying the feature must change nothing until an operator has listed their
// own hosts. Getting that list wrong locks everyone out, so it cannot be
// implicit.
func TestResellerHostGuardDisabledIsAPassThrough(t *testing.T) {
	cfg := config.CustomDomainConfig{Enabled: false}

	require.Equal(t, http.StatusOK, guardResponse(t, cfg, nil, "anything.example.com", "/api/v1/ping"))
}

func TestResellerHostGuardAdmitsCanonicalAndResellerHosts(t *testing.T) {
	cfg := config.CustomDomainConfig{Enabled: true, CanonicalHosts: []string{"api.lamtung.dev"}}
	domains := []string{"api.brand.com"}

	for _, host := range []string{
		"api.lamtung.dev",
		"API.LamTung.dev:443",
		"api.brand.com",
		"api.brand.com.",
	} {
		t.Run(host, func(t *testing.T) {
			require.Equal(t, http.StatusOK, guardResponse(t, cfg, domains, host, "/api/v1/ping"))
		})
	}
}

func TestResellerHostGuardRejectsUnknownHost(t *testing.T) {
	cfg := config.CustomDomainConfig{Enabled: true, CanonicalHosts: []string{"api.lamtung.dev"}}

	require.Equal(t, http.StatusNotFound,
		guardResponse(t, cfg, []string{"api.brand.com"}, "evil.example.com", "/api/v1/ping"))
}

// Caddy's health probe dials the upstream address directly, so its Host header
// is `localhost:8080`. Rejecting it would mark the backend unhealthy and take
// the whole deployment down — the guard would fire long before anyone noticed a
// domain was misconfigured.
func TestResellerHostGuardAdmitsLoopbackSoHealthChecksSurvive(t *testing.T) {
	cfg := config.CustomDomainConfig{Enabled: true, CanonicalHosts: []string{"api.lamtung.dev"}}

	for _, host := range []string{"localhost:8080", "127.0.0.1:8080", "[::1]:8080"} {
		t.Run(host, func(t *testing.T) {
			require.Equal(t, http.StatusOK, guardResponse(t, cfg, nil, host, "/health"))
			require.Equal(t, http.StatusOK, guardResponse(t, cfg, nil, host, "/api/v1/ping"))
		})
	}
}

// /health and the ask endpoint have to answer under any Host: the first is how
// Caddy decides the backend is alive, the second is how it decides whether a
// Host is legitimate at all. Gating the ask endpoint on the very answer it
// produces would deadlock certificate issuance.
func TestResellerHostGuardNeverBlocksHealthOrAskEndpoint(t *testing.T) {
	cfg := config.CustomDomainConfig{Enabled: true, CanonicalHosts: []string{"api.lamtung.dev"}}

	require.Equal(t, http.StatusOK, guardResponse(t, cfg, nil, "brand-new.example.com", "/health"))
	require.Equal(t, http.StatusOK, guardResponse(t, cfg, nil, "brand-new.example.com", "/internal/tls-check"))
}

// An operator who enables the guard but forgets the canonical list would lock
// out their own domain; the reseller list alone must not silently paper over it.
func TestResellerHostGuardRejectsWhenCanonicalListIsEmpty(t *testing.T) {
	cfg := config.CustomDomainConfig{Enabled: true}

	require.Equal(t, http.StatusNotFound,
		guardResponse(t, cfg, nil, "api.lamtung.dev", "/api/v1/ping"))
}
