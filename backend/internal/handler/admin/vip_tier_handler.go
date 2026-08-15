package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// VIPTierHandler handles admin VIP tier management.
type VIPTierHandler struct {
	vipTierService *service.VIPTierService
}

func NewVIPTierHandler(vipTierService *service.VIPTierService) *VIPTierHandler {
	return &VIPTierHandler{vipTierService: vipTierService}
}

// VIPTierRequest is shared by create and update. Every field is a pointer so an
// update can leave the rest of a tier alone; create fills the gaps with the
// service's defaults.
type VIPTierRequest struct {
	Level          *int     `json:"level"`
	Name           *string  `json:"name"`
	MinSpendUSD    *float64 `json:"min_spend_usd"`
	RateMultiplier *float64 `json:"rate_multiplier"`
	Concurrency    *int     `json:"concurrency"`
	GraceDays      *int     `json:"grace_days"`
	BadgeColor     *string  `json:"badge_color"`
	Enabled        *bool    `json:"enabled"`
}

func (r VIPTierRequest) toInput() service.VIPTierInput {
	return service.VIPTierInput{
		Level:          r.Level,
		Name:           r.Name,
		MinSpendUSD:    r.MinSpendUSD,
		RateMultiplier: r.RateMultiplier,
		Concurrency:    r.Concurrency,
		GraceDays:      r.GraceDays,
		BadgeColor:     r.BadgeColor,
		Enabled:        r.Enabled,
	}
}

// SetUserVIPTierRequest pins a user to a tier, or clears the pin when tier_id
// is null.
type SetUserVIPTierRequest struct {
	TierID *int64 `json:"tier_id"`
}

// List handles GET /api/v1/admin/vip-tiers
func (h *VIPTierHandler) List(c *gin.Context) {
	tiers, err := h.vipTierService.List(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.VIPTier, 0, len(tiers))
	for i := range tiers {
		out = append(out, *dto.VIPTierFromService(&tiers[i]))
	}
	response.Success(c, out)
}

// Create handles POST /api/v1/admin/vip-tiers
func (h *VIPTierHandler) Create(c *gin.Context) {
	var req VIPTierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	tier, err := h.vipTierService.Create(c.Request.Context(), req.toInput())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.VIPTierFromService(tier))
}

// Update handles PUT /api/v1/admin/vip-tiers/:id
func (h *VIPTierHandler) Update(c *gin.Context) {
	id, ok := parseVIPTierID(c)
	if !ok {
		return
	}
	var req VIPTierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	tier, err := h.vipTierService.Update(c.Request.Context(), id, req.toInput())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.VIPTierFromService(tier))
}

// Delete handles DELETE /api/v1/admin/vip-tiers/:id
func (h *VIPTierHandler) Delete(c *gin.Context) {
	id, ok := parseVIPTierID(c)
	if !ok {
		return
	}
	if err := h.vipTierService.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "VIP tier deleted successfully"})
}

// SetUserTier handles PUT /api/v1/admin/users/:id/vip-tier
func (h *VIPTierHandler) SetUserTier(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user ID")
		return
	}
	var req SetUserVIPTierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	if err := h.vipTierService.SetUserTier(c.Request.Context(), userID, req.TierID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "User VIP tier updated successfully"})
}

// GetUserStatus handles GET /api/v1/admin/users/:id/vip-tier
func (h *VIPTierHandler) GetUserStatus(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user ID")
		return
	}
	status, err := h.vipTierService.GetUserStatus(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.VIPStatusFromService(status))
}

func parseVIPTierID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid VIP tier ID")
		return 0, false
	}
	return id, true
}
