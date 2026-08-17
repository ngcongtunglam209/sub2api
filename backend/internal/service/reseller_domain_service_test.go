//go:build unit

// Tagged to match reseller_domain_branding_test.go, which shares this file's
// stub. Without the tag the two halves of one stub sat on opposite sides of a
// build constraint, and golangci-lint — which runs with no tags — saw the
// declaration of brandingState but not the tagged file calling it, so it
// reported a live helper as dead code and turned the lint job red on main.

package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubResellerDomainRepo struct {
	mu sync.Mutex
	// domains seeds hostnames with no branding override; rows seeds whole
	// snapshot rows when the test cares about the branding they carry.
	domains []string
	rows    []ActiveResellerDomain
	err     error
	calls   int

	brandingCalls  int
	lastBrandingID int64
	lastBranding   ResellerDomainBrandingUpdate

	statusCalls  int
	lastStatusID int64
	lastStatus   string
}

func (s *stubResellerDomainRepo) ListActiveDomains(context.Context) ([]ActiveResellerDomain, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	out := append([]ActiveResellerDomain(nil), s.rows...)
	for i, d := range s.domains {
		out = append(out, ActiveResellerDomain{ID: int64(i + 1), Domain: d})
	}
	return out, nil
}

func (s *stubResellerDomainRepo) UpdateBranding(_ context.Context, id int64, update ResellerDomainBrandingUpdate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.brandingCalls++
	s.lastBrandingID = id
	s.lastBranding = update
	return nil
}

func (s *stubResellerDomainRepo) brandingState() (int, int64, ResellerDomainBrandingUpdate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.brandingCalls, s.lastBrandingID, s.lastBranding
}

func (s *stubResellerDomainRepo) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func (s *stubResellerDomainRepo) setErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
}

func (s *stubResellerDomainRepo) Create(context.Context, *ResellerDomain) (*ResellerDomain, error) {
	return nil, nil
}
func (s *stubResellerDomainRepo) ListByUser(context.Context, int64) ([]*ResellerDomain, error) {
	return nil, nil
}
func (s *stubResellerDomainRepo) List(context.Context) ([]*ResellerDomain, error) { return nil, nil }
func (s *stubResellerDomainRepo) Delete(context.Context, int64) error             { return nil }

func (s *stubResellerDomainRepo) SetStatus(_ context.Context, id int64, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statusCalls++
	s.lastStatusID = id
	s.lastStatus = status
	return nil
}

func (s *stubResellerDomainRepo) statusState() (int, int64, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statusCalls, s.lastStatusID, s.lastStatus
}

// The Host header is client-controlled, so every spelling of the same name has
// to collapse to one key — otherwise an attacker picks the one that misses the
// allowlist.
func TestNormalizeDomain(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{in: "api.brand.com", want: "api.brand.com"},
		{in: "API.Brand.COM", want: "api.brand.com"},
		{in: "  api.brand.com  ", want: "api.brand.com"},
		{in: "api.brand.com.", want: "api.brand.com"},
		{in: "api.brand.com:8443", want: "api.brand.com"},
		{in: "API.Brand.COM.:443", want: "api.brand.com"},
		{in: "[2001:db8::1]:8443", want: "2001:db8::1"},
		{in: "[2001:db8::1]", want: "2001:db8::1"},
		{in: "", want: ""},
		{in: "   ", want: ""},
	} {
		t.Run(tc.in, func(t *testing.T) {
			require.Equal(t, tc.want, NormalizeDomain(tc.in))
		})
	}
}

func TestResellerDomainServiceAllowsOnlyActiveDomains(t *testing.T) {
	repo := &stubResellerDomainRepo{domains: []string{"api.brand.com", "gw.other.io"}}
	svc := NewResellerDomainService(repo, nil)

	require.True(t, svc.IsAllowedHost(context.Background(), "api.brand.com"))
	require.True(t, svc.IsAllowedHost(context.Background(), "API.Brand.com:443"))
	require.True(t, svc.IsAllowedHost(context.Background(), "gw.other.io"))
	require.False(t, svc.IsAllowedHost(context.Background(), "evil.example.com"))
	require.False(t, svc.IsAllowedHost(context.Background(), ""))
}

// The whole point of caching the set is that an unknown host — the shape a
// prober floods — costs no database round trip.
func TestResellerDomainServiceCachesTheWholeSet(t *testing.T) {
	repo := &stubResellerDomainRepo{domains: []string{"api.brand.com"}}
	svc := NewResellerDomainService(repo, nil)

	for i := 0; i < 50; i++ {
		svc.IsAllowedHost(context.Background(), "api.brand.com")
		svc.IsAllowedHost(context.Background(), "unknown.example.com")
	}

	require.Equal(t, 1, repo.callCount())
}

func TestResellerDomainServiceInvalidateForcesReload(t *testing.T) {
	repo := &stubResellerDomainRepo{domains: []string{"api.brand.com"}}
	svc := NewResellerDomainService(repo, nil)

	require.True(t, svc.IsAllowedHost(context.Background(), "api.brand.com"))
	require.Equal(t, 1, repo.callCount())

	svc.Invalidate()

	require.True(t, svc.IsAllowedHost(context.Background(), "api.brand.com"))
	require.Equal(t, 2, repo.callCount())
}

// Fail closed: everywhere else a lookup failure degrades open, but this one
// guards certificate issuance. Failing open would let anyone who can point DNS
// at us mint certificates on our Let's Encrypt account while the DB is down.
func TestResellerDomainServiceFailsClosedWithNoCachedSet(t *testing.T) {
	repo := &stubResellerDomainRepo{err: errors.New("database unavailable")}
	svc := NewResellerDomainService(repo, nil)

	require.False(t, svc.IsAllowedHost(context.Background(), "api.brand.com"))
}

// A database blip after a successful load must not take every reseller
// offline — the stale set stands until the DB comes back.
func TestResellerDomainServiceServesStaleSetOnRefreshFailure(t *testing.T) {
	repo := &stubResellerDomainRepo{domains: []string{"api.brand.com"}}
	svc := NewResellerDomainService(repo, nil)

	require.True(t, svc.IsAllowedHost(context.Background(), "api.brand.com"))

	svc.Invalidate()
	repo.setErr(errors.New("database unavailable"))

	require.True(t, svc.IsAllowedHost(context.Background(), "api.brand.com"))
	require.False(t, svc.IsAllowedHost(context.Background(), "evil.example.com"))
}

func TestValidateResellerDomain(t *testing.T) {
	require.NoError(t, validateResellerDomain("api.brand.com"))
	require.NoError(t, validateResellerDomain("a.b.c.d.example.com"))

	for _, bad := range []string{
		"",
		"localhost",       // not fully qualified
		"*.brand.com",     // a wildcard would claim subdomains another reseller may register
		"api brand.com",   // space
		"api.brand.com/x", // path
		"api..brand.com",  // empty label
	} {
		t.Run(bad, func(t *testing.T) {
			require.Error(t, validateResellerDomain(bad))
		})
	}
}

// SetStatus is the only lever the admin API has over an existing domain, and
// the status it writes decides whether the host is served at all. Anything
// outside the two known values has to be refused before it reaches the column:
// a typo like "disable" would store a status the allowlist query never matches,
// leaving the domain silently dark with nothing to explain it.
func TestSetStatusAcceptsOnlyKnownValues(t *testing.T) {
	for _, tc := range []struct {
		name     string
		in       string
		wantOK   bool
		wantSent string
	}{
		{name: "active", in: "active", wantOK: true, wantSent: "active"},
		{name: "disabled", in: "disabled", wantOK: true, wantSent: "disabled"},
		{name: "uppercase is normalized", in: "ACTIVE", wantOK: true, wantSent: "active"},
		{name: "surrounding space is trimmed", in: "  disabled  ", wantOK: true, wantSent: "disabled"},
		{name: "empty", in: ""},
		{name: "near miss", in: "disable"},
		{name: "unknown word", in: "paused"},
		{name: "sql-ish", in: "active' OR '1'='1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubResellerDomainRepo{}
			svc := NewResellerDomainService(repo, nil)

			err := svc.SetStatus(context.Background(), 7, tc.in)
			calls, id, sent := repo.statusState()

			if !tc.wantOK {
				require.Error(t, err)
				require.Zero(t, calls, "a rejected status must never reach the database")
				return
			}

			require.NoError(t, err)
			require.Equal(t, 1, calls)
			require.Equal(t, int64(7), id)
			require.Equal(t, tc.wantSent, sent)
		})
	}
}
