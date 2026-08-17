package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubResellerDomainRepo struct {
	mu      sync.Mutex
	domains []string
	err     error
	calls   int
}

func (s *stubResellerDomainRepo) ListActiveDomains(context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return append([]string(nil), s.domains...), nil
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
func (s *stubResellerDomainRepo) SetStatus(context.Context, int64, string) error  { return nil }
func (s *stubResellerDomainRepo) Delete(context.Context, int64) error             { return nil }

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
