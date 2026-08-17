package routes

import (
	"sort"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func registeredRoutes(router *gin.Engine) map[string]struct{} {
	out := map[string]struct{}{}
	for _, route := range router.Routes() {
		out[route.Method+" "+route.Path] = struct{}{}
	}
	return out
}

func requireRoutes(t *testing.T, registered map[string]struct{}, want []string, what string) {
	t.Helper()
	missing := make([]string, 0)
	for _, route := range want {
		if _, ok := registered[route]; !ok {
			missing = append(missing, route)
		}
	}
	sort.Strings(missing)
	require.Empty(t, missing, what)
}

// The store is wired in two halves that break independently: the user-facing
// catalogue and purchase routes, and the reseller-tier purchase that lives
// under /reseller-plans because that is the object being changed. A dropped
// route fails silently — the page simply stops working — so pin the contract.
func TestAddonUserRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	handlers := &handler.Handlers{Addon: handler.NewAddonHandler(nil)}
	passthrough := func(c *gin.Context) { c.Next() }
	RegisterUserRoutes(
		router.Group("/api/v1"),
		handlers,
		servermiddleware.JWTAuthMiddleware(passthrough),
		servermiddleware.AuditLogMiddleware(passthrough),
		nil,
		servermiddleware.NewPanelRateLimiter(nil, nil),
	)

	requireRoutes(t, registeredRoutes(router), []string{
		"GET /api/v1/addons",
		"POST /api/v1/addons/purchase",
		// The store lists tiers before anyone holds one, so this read has to
		// exist separately from the admin listing. Its absence is what shipped
		// a store page whose first request 404'd.
		"GET /api/v1/reseller-plans",
		"POST /api/v1/reseller-plans/:id/purchase",
	}, "add-on store routes must stay registered")
}

func TestAddonPricingAdminRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{
		AddonPricing: adminhandler.NewAddonPricingHandler(nil),
	}}
	passthrough := func(c *gin.Context) { c.Next() }
	RegisterAdminRoutes(
		router.Group("/api/v1"),
		handlers,
		servermiddleware.AdminAuthMiddleware(passthrough),
		servermiddleware.AuditLogMiddleware(passthrough),
		servermiddleware.StepUpAuthMiddleware(passthrough),
		nil,
		nil,
	)

	requireRoutes(t, registeredRoutes(router), []string{
		"GET /api/v1/admin/addon-pricing",
		"PUT /api/v1/admin/addon-pricing",
	}, "add-on pricing admin routes must stay registered")
}
