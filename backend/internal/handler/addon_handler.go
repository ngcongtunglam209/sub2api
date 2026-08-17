package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// AddonHandler is the self-service store: a user spends their own balance on
// concurrency, RPM, or a reseller tier.
//
// Every route here takes the buyer from the authenticated session and never
// from the body or the path. There is no user parameter to tamper with,
// because the only safe answer to "whose balance?" on this path is "the
// caller's".
type AddonHandler struct {
	addonService *service.AddonService
}

func NewAddonHandler(addonService *service.AddonService) *AddonHandler {
	return &AddonHandler{addonService: addonService}
}

// purchaseAddonRequest is one order.
//
// Amount and Months are plain ints with no binding tags: `required` treats 0 as
// absent and would report a missing field for what is really an out-of-range
// one. The service owns the bounds and phrases them properly.
type purchaseAddonRequest struct {
	Kind   string `json:"kind"`
	Amount int    `json:"amount"`
	Months int    `json:"months"`
}

// Catalogue handles GET /api/v1/addons.
//
// Prices, caps, and what the caller currently holds in one response: a client
// rendering "2 of 20 slots, expiring on the 3rd" should not have to make two
// calls and join them itself.
func (h *AddonHandler) Catalogue(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}

	catalogue, err := h.addonService.Catalogue(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AddonCatalogueFromService(catalogue))
}

// Purchase handles POST /api/v1/addons/purchase.
func (h *AddonHandler) Purchase(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}

	var req purchaseAddonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := h.addonService.Purchase(
		c.Request.Context(),
		subject.UserID,
		service.AddonKind(req.Kind),
		req.Amount,
		req.Months,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AddonPurchaseResultFromService(result))
}

// PurchaseResellerPlan handles POST /api/v1/reseller-plans/:id/purchase.
//
// Buying a tier runs the same assignment path an admin grant does, so the
// expiry and the credited share of the price are derived in one place. What is
// different is that the caller pays for it out of their own balance, and that
// holding an active tier already is refused — see
// AddonService.PurchaseResellerPlan for why that one is a money bug rather
// than an inconvenience.
// ListResellerPlans returns the tiers on sale.
// GET /api/v1/reseller-plans
//
// Separate from the admin listing on purpose: this one hides withdrawn tiers,
// and it is reachable by any signed-in user because the store has to show what
// is for sale before anyone holds a plan.
func (h *AddonHandler) ListResellerPlans(c *gin.Context) {
	plans, err := h.addonService.ListPurchasablePlans(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	items := make([]*dto.ResellerPlan, 0, len(plans))
	for _, plan := range plans {
		items = append(items, dto.ResellerPlanFromService(plan))
	}
	response.Success(c, items)
}

func (h *AddonHandler) PurchaseResellerPlan(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}

	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || planID <= 0 {
		response.BadRequest(c, "Invalid reseller plan ID")
		return
	}

	assignment, err := h.addonService.PurchaseResellerPlan(c.Request.Context(), subject.UserID, planID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.ResellerPlanAssignmentFromService(assignment))
}
