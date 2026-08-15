package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// VIPHandler serves a user their own VIP standing.
type VIPHandler struct {
	vipTierService *service.VIPTierService
}

func NewVIPHandler(vipTierService *service.VIPTierService) *VIPHandler {
	return &VIPHandler{vipTierService: vipTierService}
}

// Status handles GET /api/v1/vip/status
func (h *VIPHandler) Status(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	status, err := h.vipTierService.GetUserStatus(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.VIPStatusFromService(status))
}

// Tiers handles GET /api/v1/vip/tiers
//
// The whole enabled ladder, so the account page can show what the next tiers
// are worth. Disabled tiers are withheld: nobody new can reach them, and
// advertising an unreachable tier reads as a bait.
func (h *VIPHandler) Tiers(c *gin.Context) {
	tiers, err := h.vipTierService.List(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.VIPTier, 0, len(tiers))
	for i := range tiers {
		if !tiers[i].Enabled {
			continue
		}
		out = append(out, *dto.VIPTierFromService(&tiers[i]))
	}
	response.Success(c, out)
}
