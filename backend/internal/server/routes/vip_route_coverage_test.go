package routes

import (
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// The VIP surface is wired in two halves that are easy to break independently:
// tier configuration under /admin/vip-tiers, and the per-user standing under
// /admin/users/:id/vip-tier. A dropped or renamed route fails silently — the
// admin page simply stops working — so pin the contract here.
func TestVIPAdminRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{
		VIPTier: adminhandler.NewVIPTierHandler(nil),
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

	registered := map[string]struct{}{}
	for _, route := range router.Routes() {
		registered[route.Method+" "+route.Path] = struct{}{}
	}

	want := []string{
		"GET /api/v1/admin/vip-tiers",
		"POST /api/v1/admin/vip-tiers",
		"PUT /api/v1/admin/vip-tiers/:id",
		"DELETE /api/v1/admin/vip-tiers/:id",
		"GET /api/v1/admin/users/:id/vip-tier",
		"PUT /api/v1/admin/users/:id/vip-tier",
	}
	missing := make([]string, 0)
	for _, route := range want {
		if _, ok := registered[route]; !ok {
			missing = append(missing, route)
		}
	}
	sort.Strings(missing)
	require.Empty(t, missing, "VIP admin routes must stay registered")
}

// The two read-only routes the user panel calls. Registering them for real
// needs a live SettingService and panel rate limiter, so pin the declarations
// at the source level instead — the same approach the prompt-audit coverage
// test takes for gateway routes.
func TestVIPUserRoutesAreDeclared(t *testing.T) {
	routeSource, err := os.ReadFile("user.go")
	require.NoError(t, err)

	for _, declaration := range []string{
		`vip.GET("/status", h.VIP.Status)`,
		`vip.GET("/tiers", h.VIP.Tiers)`,
	} {
		require.Containsf(t, string(routeSource), declaration,
			"user-facing VIP route declaration missing: %s", declaration)
	}
}

func TestVIPAdminRoutesRejectUnauthenticatedAndNonAdminRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{
		VIPTier: adminhandler.NewVIPTierHandler(nil),
	}}
	adminAuth := servermiddleware.AdminAuthMiddleware(func(c *gin.Context) {
		if c.GetHeader("Authorization") == "" {
			servermiddleware.AbortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authorization required")
			return
		}
		servermiddleware.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Admin access required")
	})
	passthrough := func(c *gin.Context) { c.Next() }
	RegisterAdminRoutes(
		router.Group("/api/v1"),
		handlers,
		adminAuth,
		servermiddleware.AuditLogMiddleware(passthrough),
		servermiddleware.StepUpAuthMiddleware(passthrough),
		nil,
		nil,
	)

	for _, target := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/admin/vip-tiers"},
		{http.MethodGet, "/api/v1/admin/users/1/vip-tier"},
	} {
		for _, tc := range []struct {
			name       string
			auth       string
			wantStatus int
		}{
			{name: "unauthenticated", wantStatus: http.StatusUnauthorized},
			{name: "non-admin", auth: "Bearer user-token", wantStatus: http.StatusForbidden},
		} {
			t.Run(tc.name+" "+target.method+" "+target.path, func(t *testing.T) {
				recorder := httptest.NewRecorder()
				request := httptest.NewRequest(target.method, target.path, nil)
				if tc.auth != "" {
					request.Header.Set("Authorization", tc.auth)
				}
				router.ServeHTTP(recorder, request)
				require.Equal(t, tc.wantStatus, recorder.Code)
			})
		}
	}
}
