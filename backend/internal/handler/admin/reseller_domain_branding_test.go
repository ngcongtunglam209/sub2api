//go:build unit

package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type brandingStubRepo struct {
	brandingCalls int
	lastBranding  service.ResellerDomainBrandingUpdate
	statusCalls   int
	lastStatus    string
}

func (s *brandingStubRepo) ListActiveDomains(context.Context) ([]service.ActiveResellerDomain, error) {
	return nil, nil
}

func (s *brandingStubRepo) Create(context.Context, *service.ResellerDomain) (*service.ResellerDomain, error) {
	return nil, nil
}

func (s *brandingStubRepo) ListByUser(context.Context, int64) ([]*service.ResellerDomain, error) {
	return nil, nil
}

func (s *brandingStubRepo) List(context.Context) ([]*service.ResellerDomain, error) { return nil, nil }
func (s *brandingStubRepo) Delete(context.Context, int64) error                     { return nil }

func (s *brandingStubRepo) SetStatus(_ context.Context, _ int64, status string) error {
	s.statusCalls++
	s.lastStatus = status
	return nil
}

func (s *brandingStubRepo) UpdateBranding(_ context.Context, _ int64, update service.ResellerDomainBrandingUpdate) error {
	s.brandingCalls++
	s.lastBranding = update
	return nil
}

func patchDomain(t *testing.T, body string) (*brandingStubRepo, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	repo := &brandingStubRepo{}
	handler := NewResellerHandler(nil, service.NewResellerDomainService(repo, nil))

	r := gin.New()
	r.PATCH("/reseller-domains/:id", handler.SetDomainStatus)

	req := httptest.NewRequest(http.MethodPatch, "/reseller-domains/7", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return repo, w
}

// The existing status-only request keeps working untouched — the admin UI and
// anything scripted against it predate the branding fields.
func TestPatchDomainStillAcceptsStatusAlone(t *testing.T) {
	repo, w := patchDomain(t, `{"status":"disabled"}`)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, repo.statusCalls)
	require.Equal(t, "disabled", repo.lastStatus)
	require.Zero(t, repo.brandingCalls, "a status-only request must not touch the branding")
}

func TestPatchDomainAcceptsBrandingAlongsideStatus(t *testing.T) {
	repo, w := patchDomain(t, `{"status":"active","site_name":"Brand","site_subtitle":"Brand Gateway"}`)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, repo.statusCalls)
	require.Equal(t, 1, repo.brandingCalls)
	require.NotNil(t, repo.lastBranding.SiteName)
	require.Equal(t, "Brand", *repo.lastBranding.SiteName)
	require.NotNil(t, repo.lastBranding.SiteSubtitle)
	require.Equal(t, "Brand Gateway", *repo.lastBranding.SiteSubtitle)
	require.Nil(t, repo.lastBranding.SiteLogo, "an omitted field must be left alone, not cleared")
}

// Branding without a status is a legitimate edit: renaming a domain must not
// require restating whether it is switched on.
func TestPatchDomainAcceptsBrandingAlone(t *testing.T) {
	repo, w := patchDomain(t, `{"site_name":"Brand"}`)

	require.Equal(t, http.StatusOK, w.Code)
	require.Zero(t, repo.statusCalls)
	require.Equal(t, 1, repo.brandingCalls)
}

// Empty clears the override and puts the domain back on the deployment's own
// branding — the only way back, so it must not be mistaken for "unset".
func TestPatchDomainTreatsEmptyStringAsClear(t *testing.T) {
	repo, w := patchDomain(t, `{"site_name":"","site_logo":""}`)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, repo.brandingCalls)
	require.NotNil(t, repo.lastBranding.SiteName)
	require.Equal(t, "", *repo.lastBranding.SiteName)
	require.NotNil(t, repo.lastBranding.SiteLogo)
	require.Equal(t, "", *repo.lastBranding.SiteLogo)
	require.Nil(t, repo.lastBranding.SiteSubtitle)
}

func TestPatchDomainRejectsBadInput(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "unknown status", body: `{"status":"paused"}`},
		{name: "nothing to update", body: `{}`},
		{name: "over-long site name", body: `{"site_name":"` + strings.Repeat("x", 101) + `"}`},
		// A JSON-escaped NUL: it survives HTML escaping intact and
		// never appears in a name anyone meant to type.
		{name: "control characters", body: `{"site_name":"Bra\u0000nd"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, w := patchDomain(t, tc.body)

			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Zero(t, repo.brandingCalls, "a rejected request must not reach the database")
			require.Zero(t, repo.statusCalls)
		})
	}
}
