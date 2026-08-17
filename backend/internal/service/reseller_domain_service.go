package service

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/branding"
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
	// domains is a map rather than a slice: lookup happens on every proxied
	// request, so it must not be linear in the number of resellers.
	//
	// The value is the branding to render under, not an empty struct: the allow
	// decision and the branding are read on the same request, from the same
	// row, so one snapshot answers both without a second query.
	domains   map[string]branding.Host
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

	// onInvalidate lets the rendered-HTML cache hear about a domain edit.
	//
	// Without it, changing a reseller's name would expire this snapshot and
	// change nothing a visitor sees: the HTML for that domain is cached under
	// its own key and, unlike the settings cache, has no TTL to fall back on —
	// the old name would survive until the next global settings change or a
	// restart, which is exactly when an operator concludes the feature is
	// broken and edits it again.
	onInvalidate atomic.Value // func()

	cache atomic.Value // resellerDomainSnapshot
	sf    singleflight.Group
}

// SetOnInvalidateCallback registers a listener for domain-set changes, mirroring
// SettingService.SetOnUpdateCallback. Optional: unset, nothing downstream is
// notified and behaviour is unchanged.
func (s *ResellerDomainService) SetOnInvalidateCallback(callback func()) {
	if s == nil || callback == nil {
		return
	}
	s.onInvalidate.Store(callback)
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

// ResolveHostBranding returns the branding this hostname renders under.
//
// Falling back is the default and the failure mode: the canonical host, an
// unknown host, a domain with nothing configured, and a database that is down
// all return the zero Host, which every consumer reads as "use the global
// settings". Branding is cosmetic — unlike IsAllowedHost, which gates
// certificate issuance and therefore fails closed, there is nothing here worth
// serving an error page over.
func (s *ResellerDomainService) ResolveHostBranding(ctx context.Context, host string) branding.Host {
	if s == nil || s.repo == nil {
		return branding.Host{}
	}

	domain := NormalizeDomain(host)
	if domain == "" {
		return branding.Host{}
	}

	// The deployment's own hostnames render the deployment's own branding, and
	// answer without touching the database — same reasoning as IsAllowedHost.
	if _, ok := s.canonical[domain]; ok {
		return branding.Host{}
	}

	snapshot, err := s.activeSnapshot(ctx)
	if err != nil {
		return branding.Host{}
	}

	return snapshot.domains[domain]
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

		set := make(map[string]branding.Host, len(domains))
		for _, d := range domains {
			normalized := NormalizeDomain(d.Domain)
			if normalized == "" {
				continue
			}
			// Only rows that actually override something carry an identity of
			// their own; the rest resolve to the zero Host and therefore share
			// the default HTML cache entry with everyone else.
			host := branding.Host{
				DomainID:     d.ID,
				SiteName:     strings.TrimSpace(d.SiteName),
				SiteLogo:     strings.TrimSpace(d.SiteLogo),
				SiteSubtitle: strings.TrimSpace(d.SiteSubtitle),
			}
			if !host.HasOverride() {
				host = branding.Host{}
			}
			set[normalized] = host
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

	if callback, ok := s.onInvalidate.Load().(func()); ok && callback != nil {
		callback()
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

// Branding field caps. Names and subtitles are chrome, not content: the column
// widths in migration 229 are the hard limit, and these keep a paste of an
// entire document from reaching them. The logo allowance is wide because an
// inline data: URI is a legitimate value — the same latitude the global
// site_logo setting has.
const (
	maxResellerSiteNameLen     = 100
	maxResellerSiteSubtitleLen = 200
	maxResellerSiteLogoLen     = 8192
)

// UpdateBranding edits what a single hostname renders as.
//
// An empty string clears the override rather than blanking the panel: the
// stored value and "unset" are the same state, so an operator undoing a
// customisation puts the domain back on the deployment's own branding instead
// of onto a nameless one.
func (s *ResellerDomainService) UpdateBranding(ctx context.Context, id int64, update ResellerDomainBrandingUpdate) error {
	if id <= 0 {
		return infraerrors.BadRequest("INVALID_RESELLER_DOMAIN", "a reseller domain id is required")
	}
	if update.IsEmpty() {
		return nil
	}

	normalized := ResellerDomainBrandingUpdate{}
	for _, field := range []struct {
		in     *string
		out    **string
		max    int
		label  string
		errKey string
	}{
		{in: update.SiteName, out: &normalized.SiteName, max: maxResellerSiteNameLen, label: "site_name", errKey: "INVALID_RESELLER_SITE_NAME"},
		{in: update.SiteLogo, out: &normalized.SiteLogo, max: maxResellerSiteLogoLen, label: "site_logo", errKey: "INVALID_RESELLER_SITE_LOGO"},
		{in: update.SiteSubtitle, out: &normalized.SiteSubtitle, max: maxResellerSiteSubtitleLen, label: "site_subtitle", errKey: "INVALID_RESELLER_SITE_SUBTITLE"},
	} {
		if field.in == nil {
			continue
		}
		value := strings.TrimSpace(*field.in)
		if err := validateResellerBrandingValue(value, field.max, field.label, field.errKey); err != nil {
			return err
		}
		v := value
		*field.out = &v
	}

	if err := s.repo.UpdateBranding(ctx, id, normalized); err != nil {
		return err
	}

	// Same snapshot serves the allow decision and the branding, so a branding
	// edit has to expire it too — otherwise the operator's change appears to
	// have done nothing for up to a minute.
	s.Invalidate()
	return nil
}

// validateResellerBrandingValue rejects what would either overflow the column
// or render as something other than the text the operator typed.
func validateResellerBrandingValue(value string, max int, label, errKey string) error {
	if len(value) > max {
		return infraerrors.BadRequest(errKey, fmt.Sprintf("%s exceeds %d characters", label, max))
	}
	// Control characters never appear in a legitimate name, logo URL or
	// subtitle, and they are the part of an injected string that survives HTML
	// escaping intact. Refuse them at the door rather than at every render.
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return infraerrors.BadRequest(errKey, fmt.Sprintf("%s contains control characters", label))
		}
	}
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
