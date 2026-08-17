// Package branding carries the per-host branding identity that a request was
// resolved to, from the middleware that resolves it to everything that renders
// under it.
//
// It is a leaf package on purpose. The resolver lives in `service`, the
// injection point lives in `service`, the HTML cache lives in `web`, and the
// middleware lives in `server/middleware`; a shared type in any of those would
// drag one of them into the others.
package branding

import (
	"context"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

// DefaultCacheKey names the deployment's own branding — the global settings,
// unmodified. Canonical hosts, unknown hosts, health checks and every domain
// without an override all render under this one key.
const DefaultCacheKey = "default"

// Host is the branding a single hostname renders under.
//
// Every field is optional and falls back **independently**: a reseller who
// overrides only the name keeps the deployment's logo and subtitle. Empty means
// "use the global setting", never "render blank" — which is what makes an
// unconfigured domain indistinguishable from today's behaviour.
type Host struct {
	// DomainID is the reseller_domains row this branding came from. It is the
	// identity the HTML cache is keyed by.
	//
	// The raw Host header is deliberately NOT that key: it is attacker
	// controlled, so keying on it would let anyone grow the cache without bound
	// by sending a million distinct hostnames. Resolve first, key second — a
	// host that resolves to nothing lands on DefaultCacheKey along with
	// everybody else.
	DomainID int64

	SiteName     string
	SiteLogo     string
	SiteSubtitle string
}

// HasOverride reports whether this host renders as anything other than the
// deployment's own branding.
func (h Host) HasOverride() bool {
	return h.SiteName != "" || h.SiteLogo != "" || h.SiteSubtitle != ""
}

// CacheKey is the identity rendered HTML may be cached under.
//
// A host with nothing overridden collapses onto DefaultCacheKey rather than
// taking a slot of its own: its output is byte-for-byte the deployment's, so a
// separate entry would only cost memory. That also bounds the number of live
// keys by the number of *configured* domains rather than by the number of
// distinct Host headers seen.
func (h Host) CacheKey() string {
	if h.DomainID <= 0 || !h.HasOverride() {
		return DefaultCacheKey
	}
	return "domain:" + strconv.FormatInt(h.DomainID, 10)
}

// NewContext stores the resolved branding for the rest of the request.
func NewContext(ctx context.Context, host Host) context.Context {
	return context.WithValue(ctx, ctxkey.HostBranding, host)
}

// FromContext returns the branding resolved for this request, or the zero Host
// — meaning "the deployment's own branding" — when nothing resolved it.
//
// Falling back is the default everywhere: a context that never passed through
// the middleware (a background job, a test, a health check) must render exactly
// what it rendered before this feature existed.
func FromContext(ctx context.Context) Host {
	if ctx == nil {
		return Host{}
	}
	host, _ := ctx.Value(ctxkey.HostBranding).(Host)
	return host
}
