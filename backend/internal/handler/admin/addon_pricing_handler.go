package admin

import (
	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// AddonPricingHandler lets an operator reprice the self-service store without
// a deploy.
//
// The values live in the settings KV rather than a table of their own — see
// the constants in addon_pricing_service.go. Four scalars do not earn a
// schema, and repricing deliberately leaves existing holders alone, exactly as
// editing a reseller tier does.
type AddonPricingHandler struct {
	pricingService *service.AddonPricingService
}

func NewAddonPricingHandler(pricingService *service.AddonPricingService) *AddonPricingHandler {
	return &AddonPricingHandler{pricingService: pricingService}
}

// UpdateAddonPricingRequest is an operator edit. Every field is a pointer so
// raising one price does not require resending the caps and risking a
// write-back of a value read minutes ago.
type UpdateAddonPricingRequest struct {
	ConcurrencyUnitPrice *float64 `json:"concurrency_unit_price"`
	ConcurrencyMax       *int     `json:"concurrency_max"`
	RPMUnitPrice         *float64 `json:"rpm_unit_price"`
	RPMMax               *int     `json:"rpm_max"`
}

func (r UpdateAddonPricingRequest) toUpdate() service.UpdateAddonPricingRequest {
	return service.UpdateAddonPricingRequest{
		ConcurrencyUnitPrice: r.ConcurrencyUnitPrice,
		ConcurrencyMax:       r.ConcurrencyMax,
		RPMUnitPrice:         r.RPMUnitPrice,
		RPMMax:               r.RPMMax,
	}
}

// Get handles GET /api/v1/admin/addon-pricing
func (h *AddonPricingHandler) Get(c *gin.Context) {
	pricing, err := h.pricingService.Get(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AddonPricingFromService(pricing))
}

// Update handles PUT /api/v1/admin/addon-pricing
func (h *AddonPricingHandler) Update(c *gin.Context) {
	var req UpdateAddonPricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Bounds are checked in the service, not here: a price of zero gives
	// concurrency away and a cap that is not a whole number of RPM blocks can
	// never be reached, and both must hold for every caller rather than for
	// whoever remembers to ask.
	pricing, err := h.pricingService.Update(c.Request.Context(), req.toUpdate())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AddonPricingFromService(pricing))
}
