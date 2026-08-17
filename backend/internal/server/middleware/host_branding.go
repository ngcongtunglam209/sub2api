package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/branding"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// HostBranding resolves the Host header to a reseller's branding once, at the
// front of the chain, and puts the answer in the request context.
//
// Once, because everything downstream needs the same answer: the public
// settings endpoint, the settings injected into index.html, the <title>, and
// the favicon. Resolving per consumer would mean four lookups per page load and
// four chances for them to disagree with each other.
//
// The result is a snapshot lookup, not a query — ResolveHostBranding reads the
// same cached set the host allowlist already reads — so this costs a map lookup
// on the request path, including on the gateway paths that never render HTML.
//
// Gated on the same custom_domain.enabled switch as ResellerHostGuard: with
// custom domains off, no host but this deployment's own is served, so there is
// no second branding to resolve and no reason to touch the database.
func HostBranding(cfg config.CustomDomainConfig, resellerDomainService *service.ResellerDomainService) gin.HandlerFunc {
	if !cfg.Enabled || resellerDomainService == nil {
		return func(c *gin.Context) { c.Next() }
	}

	return func(c *gin.Context) {
		// The health probe and the TLS ask endpoint answer with a status code,
		// never with branding, and the ask endpoint is the path Caddy hits for
		// every unknown host during a handshake — the one an attacker floods.
		// Neither has a brand to render, so neither pays for resolving one.
		if path := c.Request.URL.Path; path == "/health" || strings.HasPrefix(path, "/internal/") {
			c.Next()
			return
		}

		host := resellerDomainService.ResolveHostBranding(c.Request.Context(), c.Request.Host)
		if !host.HasOverride() {
			// Nothing resolved: leave the context untouched so every reader
			// falls back to the global settings, exactly as before this
			// middleware existed. Health checks and the TLS ask endpoint land
			// here too, which is why they are not special-cased.
			c.Next()
			return
		}

		c.Request = c.Request.WithContext(branding.NewContext(c.Request.Context(), host))
		c.Next()
	}
}
