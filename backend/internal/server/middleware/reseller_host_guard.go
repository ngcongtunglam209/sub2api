package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// loopbackHosts are the names a request can arrive under when nothing external
// is involved: Caddy's own health probe dials the upstream address directly, so
// its Host header is `localhost:8080`, not any configured domain.
//
// Rejecting these would mark the backend unhealthy and take the whole
// deployment down — the guard would fire before anyone noticed a domain was
// misconfigured.
var loopbackHosts = map[string]struct{}{
	"localhost": {},
	"127.0.0.1": {},
	"[::1]":     {},
	"::1":       {},
}

// ResellerHostGuard rejects requests arriving under a hostname that is neither
// this deployment's own domain nor an active reseller's.
//
// It exists because the `on_demand_tls ask` endpoint only gates *certificate
// issuance*. Once a certificate is issued it stays valid for about 90 days, so
// disabling a reseller would otherwise do nothing at all until it expired.
// Turning a reseller off has to cut traffic, not just future certificates.
//
// Disabled by default: with `custom_domain.enabled` false this returns a
// pass-through, so deploying the feature changes no behaviour until an operator
// has listed their canonical hosts. Getting that list wrong locks everyone out,
// which is not a mistake to make implicitly.
func ResellerHostGuard(cfg config.CustomDomainConfig, resellerDomainService *service.ResellerDomainService) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) { c.Next() }
	}

	return func(c *gin.Context) {
		// Health and the ask endpoint itself must answer regardless of Host:
		// the first is how Caddy decides the backend is alive, and the second
		// is how it decides whether a Host is legitimate in the first place.
		path := c.Request.URL.Path
		if path == "/health" || strings.HasPrefix(path, "/internal/") {
			c.Next()
			return
		}

		host := service.NormalizeDomain(c.Request.Host)
		if host == "" {
			c.Next()
			return
		}
		if _, ok := loopbackHosts[host]; ok {
			c.Next()
			return
		}
		// The service owns both lists — canonical hosts from config and active
		// resellers from the database. Keeping a second copy of the canonical
		// set here is how the guard and the certificate check drift apart.
		if resellerDomainService.IsAllowedHost(c.Request.Context(), host) {
			c.Next()
			return
		}

		// Plain 404 with no body: a disabled reseller's customers should see
		// "nothing here", not a message revealing that the host was once valid
		// or that some other service is answering on this address.
		c.AbortWithStatus(http.StatusNotFound)
	}
}
