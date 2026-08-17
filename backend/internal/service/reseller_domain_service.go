package service

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const (
	// resellerDomainCacheTTL bounds how stale the allowlist may get. A newly
	// added domain therefore takes up to this long to start serving, which is
	// far shorter than the DNS propagation the reseller is waiting on anyway.
	resellerDomainCacheTTL = 60 * time.Second

	// resellerDomainErrorTTL keeps a failed refresh from hammering the database
	// once per request while still retrying often enough to recover quickly.
	resellerDomainErrorTTL = 5 * time.Second
)

type resellerDomainSnapshot struct {
	// domains is a set rather than a slice: lookup happens on every proxied
	// request, so it must not be linear in the number of resellers.
	domains   map[string]struct{}
	expiresAt time.Time
}

// ResellerDomainService answers "may this hostname be served?" fast enough to
// sit on both the TLS handshake path and the request path.
//
// The whole active set is cached as one snapshot. That is deliberate: the row
// count is operator-scale, and holding the complete set means an unknown host —
// the shape an attacker probing for open certificate issuance produces — is
// denied from memory without touching the database at all.
type ResellerDomainService struct {
	repo ResellerDomainRepository

	// canonical holds this deployment's own hostnames, from config rather than
	// the database on purpose: the shipped edge config routes every host —
	// including the primary domain — through the ask endpoint, so answering for
	// the primary domain must not depend on the database being reachable.
	canonical map[string]struct{}

	quotaResolver ResellerDomainQuotaResolver

	cache atomic.Value // resellerDomainSnapshot
	sf    singleflight.Group
}

// checkDomainQuota refuses a domain the user's plan does not cover.
//
// Counts every row the user owns, disabled ones included: a disabled domain
// still holds its certificate and can be switched back on, so letting it fall
// out of the count would make the quota trivial to walk around.
func (s *ResellerDomainService) checkDomainQuota(ctx context.Context, userID int64) error {
	if s.quotaResolver == nil {
		return nil
	}

	maxDomains, err := s.quotaResolver.MaxDomainsForUser(ctx, userID)
	if err != nil {
		return err
	}
	if maxDomains <= 0 {
		return infraerrors.Forbidden("RESELLER_PLAN_REQUIRED", "an active reseller plan is required to register a custom domain")
	}

	existing, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return err
	}
	if len(existing) >= maxDomains {
		return infraerrors.Forbidden("RESELLER_DOMAIN_QUOTA_EXCEEDED",
			fmt.Sprintf("this reseller plan allows %d custom domain(s)", maxDomains))
	}
	return nil
}

// ProvideResellerDomainService adapts the constructor for wire, which cannot
// inject a bare []string, and attaches the per-plan domain quota.
func ProvideResellerDomainService(repo ResellerDomainRepository, planService *ResellerPlanService, cfg *config.Config) *ResellerDomainService {
	svc := NewResellerDomainService(repo, cfg.CustomDomain.CanonicalHosts)
	svc.SetQuotaResolver(planService)
	return svc
}

func NewResellerDomainService(repo ResellerDomainRepository, canonicalHosts []string) *ResellerDomainService {
	canonical := make(map[string]struct{}, len(canonicalHosts))
	for _, host := range canonicalHosts {
		if normalized := NormalizeDomain(host); normalized != "" {
			canonical[normalized] = struct{}{}
		}
	}
	return &ResellerDomainService{repo: repo, canonical: canonical}
}

// IsAllowedHost reports whether this deployment will serve the hostname —
// either its own canonical domain or an active reseller's.
func (s *ResellerDomainService) IsAllowedHost(ctx context.Context, host string) bool {
	if s == nil {
		return false
	}

	domain := NormalizeDomain(host)
	if domain == "" {
		return false
	}

	// Canonical hosts answer from config, before any database work: this path
	// also renews the primary domain's certificate, and coupling that to the
	// database would turn a database outage into an expired certificate.
	if _, ok := s.canonical[domain]; ok {
		return true
	}

	if s.repo == nil {
		return false
	}

	snapshot, err := s.activeSnapshot(ctx)
	if err != nil {
		// Fail closed. Everywhere else in this codebase a lookup failure
		// degrades open, but this one guards certificate issuance: failing open
		// would let anyone who can point DNS at us mint certificates on our
		// Let's Encrypt account while the database is down.
		logger.LegacyPrintf("service.resellerdomain", "[ResellerDomain] allowlist unavailable, denying %q: %v", domain, err)
		return false
	}

	_, ok := snapshot.domains[domain]
	return ok
}

// activeSnapshot returns the cached set, refreshing it at most once at a time.
func (s *ResellerDomainService) activeSnapshot(ctx context.Context) (resellerDomainSnapshot, error) {
	if cached, ok := s.cache.Load().(resellerDomainSnapshot); ok && time.Now().Before(cached.expiresAt) {
		return cached, nil
	}

	result, err, _ := s.sf.Do("reseller-domains", func() (any, error) {
		// Re-check inside the flight: the winner of a race has already paid for
		// the query by the time the losers arrive.
		if cached, ok := s.cache.Load().(resellerDomainSnapshot); ok && time.Now().Before(cached.expiresAt) {
			return cached, nil
		}

		// WithoutCancel so a client that hangs up mid-refresh cannot poison the
		// cache for everyone queued behind it.
		domains, err := s.repo.ListActiveDomains(context.WithoutCancel(ctx))
		if err != nil {
			// Serve the stale set if we have one — a database blip should not
			// take every reseller offline — but shorten the retry window.
			if cached, ok := s.cache.Load().(resellerDomainSnapshot); ok && cached.domains != nil {
				cached.expiresAt = time.Now().Add(resellerDomainErrorTTL)
				s.cache.Store(cached)
				return cached, nil
			}
			return resellerDomainSnapshot{}, err
		}

		set := make(map[string]struct{}, len(domains))
		for _, d := range domains {
			if normalized := NormalizeDomain(d); normalized != "" {
				set[normalized] = struct{}{}
			}
		}

		snapshot := resellerDomainSnapshot{domains: set, expiresAt: time.Now().Add(resellerDomainCacheTTL)}
		s.cache.Store(snapshot)
		return snapshot, nil
	})
	if err != nil {
		return resellerDomainSnapshot{}, err
	}

	snapshot, ok := result.(resellerDomainSnapshot)
	if !ok {
		return resellerDomainSnapshot{}, fmt.Errorf("unexpected reseller domain cache type %T", result)
	}
	return snapshot, nil
}

// Invalidate drops the cached set so the next lookup reloads.
//
// Single-node only: another instance keeps its own snapshot until its TTL
// lapses. That is tolerable precisely because the TTL is short — adding a
// pub/sub round trip to save 60 seconds on an operator action is not worth the
// moving parts.
func (s *ResellerDomainService) Invalidate() {
	if s == nil {
		return
	}

	// Expire in place rather than discarding the set. Dropping it would throw
	// away the fallback the error path depends on, so an operator edit followed
	// by one bad database moment would take every reseller offline — the moment
	// an edit happens is exactly when that is least acceptable.
	if cached, ok := s.cache.Load().(resellerDomainSnapshot); ok && cached.domains != nil {
		cached.expiresAt = time.Time{}
		s.cache.Store(cached)
		return
	}
	s.cache.Store(resellerDomainSnapshot{})
}

// ResellerDomainQuotaResolver reports how many domains a user may hold.
//
// A narrow interface rather than the plan service itself: this package only
// needs the number, and depending on the whole service would tie the domain
// allowlist to the plan lifecycle for no gain.
type ResellerDomainQuotaResolver interface {
	MaxDomainsForUser(ctx context.Context, userID int64) (int, error)
}

// SetQuotaResolver wires the per-plan domain quota. Left unset, domain count
// is unlimited — which is the behaviour before reseller plans existed, and the
// right default for a deployment that never adopts them.
func (s *ResellerDomainService) SetQuotaResolver(resolver ResellerDomainQuotaResolver) {
	if s != nil {
		s.quotaResolver = resolver
	}
}

func (s *ResellerDomainService) Create(ctx context.Context, domain string, userID int64, notes string) (*ResellerDomain, error) {
	normalized := NormalizeDomain(domain)
	if err := validateResellerDomain(normalized); err != nil {
		return nil, err
	}
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_RESELLER_DOMAIN_USER", "a reseller domain must belong to a user")
	}

	if err := s.checkDomainQuota(ctx, userID); err != nil {
		return nil, err
	}

	created, err := s.repo.Create(ctx, &ResellerDomain{
		Domain: normalized,
		UserID: userID,
		Status: ResellerDomainStatusActive,
		Notes:  strings.TrimSpace(notes),
	})
	if err != nil {
		return nil, err
	}

	s.Invalidate()
	return created, nil
}

func (s *ResellerDomainService) List(ctx context.Context) ([]*ResellerDomain, error) {
	return s.repo.List(ctx)
}

func (s *ResellerDomainService) ListByUser(ctx context.Context, userID int64) ([]*ResellerDomain, error) {
	return s.repo.ListByUser(ctx, userID)
}

func (s *ResellerDomainService) SetStatus(ctx context.Context, id int64, status string) error {
	status = strings.ToLower(strings.TrimSpace(status))
	if status != ResellerDomainStatusActive && status != "disabled" {
		return infraerrors.BadRequest("INVALID_RESELLER_DOMAIN_STATUS", "status must be active or disabled")
	}
	if err := s.repo.SetStatus(ctx, id, status); err != nil {
		return err
	}
	s.Invalidate()
	return nil
}

func (s *ResellerDomainService) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.Invalidate()
	return nil
}

// validateResellerDomain rejects shapes that would either never work or would
// hand someone else's traffic to a reseller.
func validateResellerDomain(domain string) error {
	if domain == "" {
		return infraerrors.BadRequest("INVALID_RESELLER_DOMAIN", "domain is required")
	}
	if len(domain) > 253 {
		return infraerrors.BadRequest("INVALID_RESELLER_DOMAIN", "domain exceeds 253 characters")
	}
	// A wildcard would let one reseller claim every subdomain of a zone,
	// including ones another reseller might later register.
	if strings.ContainsAny(domain, "*/ \t") {
		return infraerrors.BadRequest("INVALID_RESELLER_DOMAIN", "domain must be a single hostname, without wildcards, spaces or paths")
	}
	if !strings.Contains(domain, ".") {
		return infraerrors.BadRequest("INVALID_RESELLER_DOMAIN", "domain must be fully qualified")
	}
	for _, label := range strings.Split(domain, ".") {
		if label == "" {
			return infraerrors.BadRequest("INVALID_RESELLER_DOMAIN", "domain contains an empty label")
		}
		if len(label) > 63 {
			return infraerrors.BadRequest("INVALID_RESELLER_DOMAIN", "domain label exceeds 63 characters")
		}
	}
	return nil
}
