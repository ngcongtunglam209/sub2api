package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ResellerDomainHandler serves the endpoint Caddy's `on_demand_tls ask`
// consults before issuing a certificate for an unrecognised host.
type ResellerDomainHandler struct {
	resellerDomainService *service.ResellerDomainService
}

func NewResellerDomainHandler(resellerDomainService *service.ResellerDomainService) *ResellerDomainHandler {
	return &ResellerDomainHandler{resellerDomainService: resellerDomainService}
}

// TLSCheck answers the certificate-issuance question and nothing else.
// GET /internal/tls-check?domain=api.brand.com
//
// Caddy treats any 2xx as permission to obtain a certificate and any other
// status as refusal, so the body is irrelevant and deliberately empty — this
// runs on every TLS handshake with an unknown host, which is precisely the
// request an attacker floods when probing for open issuance.
//
// It lives under `/internal/` rather than `/api/v1/` for two reasons: that
// prefix is trivial to block at the edge (see deploy/Caddyfile), and paths
// under `/api/v1/` pay a crypto/rand CSP nonce per request that this endpoint
// has no use for.
func (h *ResellerDomainHandler) TLSCheck(c *gin.Context) {
	if h.resellerDomainService == nil {
		c.Status(http.StatusForbidden)
		return
	}

	if h.resellerDomainService.IsAllowedHost(c.Request.Context(), c.Query("domain")) {
		c.Status(http.StatusOK)
		return
	}

	// 403 rather than 404: there is nothing to discover here, and Caddy only
	// distinguishes 2xx from everything else.
	c.Status(http.StatusForbidden)
}
