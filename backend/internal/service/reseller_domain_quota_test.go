package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// quotaStubRepo records creates and reports whatever the test seeds as existing.
type quotaStubRepo struct {
	existing []*ResellerDomain
	created  int
}

func (s *quotaStubRepo) ListActiveDomains(context.Context) ([]string, error) { return nil, nil }

func (s *quotaStubRepo) Create(_ context.Context, d *ResellerDomain) (*ResellerDomain, error) {
	s.created++
	return d, nil
}

func (s *quotaStubRepo) ListByUser(context.Context, int64) ([]*ResellerDomain, error) {
	return s.existing, nil
}

func (s *quotaStubRepo) List(context.Context) ([]*ResellerDomain, error) { return nil, nil }
func (s *quotaStubRepo) SetStatus(context.Context, int64, string) error  { return nil }
func (s *quotaStubRepo) Delete(context.Context, int64) error             { return nil }

type stubQuotaResolver struct{ maxDomains int }

func (s stubQuotaResolver) MaxDomainsForUser(context.Context, int64) (int, error) {
	return s.maxDomains, nil
}

// Without a resolver the count is unlimited — the behaviour before reseller
// plans existed, and the right default for a deployment that never adopts them.
func TestResellerDomainCreateUnlimitedWithoutQuotaResolver(t *testing.T) {
	repo := &quotaStubRepo{existing: []*ResellerDomain{{ID: 1}, {ID: 2}, {ID: 3}}}
	svc := NewResellerDomainService(repo, nil)

	_, err := svc.Create(context.Background(), "api.brand.com", 42, "")
	require.NoError(t, err)
	require.Equal(t, 1, repo.created)
}

// No active plan means no custom domains at all: otherwise any ordinary account
// could grow the allowlist that gates certificate issuance.
func TestResellerDomainCreateRequiresAnActivePlan(t *testing.T) {
	repo := &quotaStubRepo{}
	svc := NewResellerDomainService(repo, nil)
	svc.SetQuotaResolver(stubQuotaResolver{maxDomains: 0})

	_, err := svc.Create(context.Background(), "api.brand.com", 42, "")
	require.Error(t, err)
	require.Equal(t, 0, repo.created)
}

func TestResellerDomainCreateEnforcesPlanQuota(t *testing.T) {
	repo := &quotaStubRepo{existing: []*ResellerDomain{{ID: 1}, {ID: 2}, {ID: 3}}}
	svc := NewResellerDomainService(repo, nil)
	svc.SetQuotaResolver(stubQuotaResolver{maxDomains: 3})

	_, err := svc.Create(context.Background(), "fourth.brand.com", 42, "")
	require.Error(t, err)
	require.Equal(t, 0, repo.created)
}

// Disabled rows still count. A disabled domain keeps its certificate and can be
// switched back on, so dropping it from the tally makes the quota trivial to
// walk around.
func TestResellerDomainQuotaCountsDisabledDomains(t *testing.T) {
	repo := &quotaStubRepo{existing: []*ResellerDomain{
		{ID: 1, Status: ResellerDomainStatusActive},
		{ID: 2, Status: "disabled"},
	}}
	svc := NewResellerDomainService(repo, nil)
	svc.SetQuotaResolver(stubQuotaResolver{maxDomains: 2})

	_, err := svc.Create(context.Background(), "third.brand.com", 42, "")
	require.Error(t, err)
	require.Equal(t, 0, repo.created)
}
