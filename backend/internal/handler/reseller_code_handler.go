package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ResellerCodeHandler lets a reseller turn their own balance into redeem codes.
type ResellerCodeHandler struct {
	redeemService    *service.RedeemService
	resellerPlanSvc  *service.ResellerPlanService
	resellerCodeRepo service.ResellerCodeRepository
}

func NewResellerCodeHandler(
	redeemService *service.RedeemService,
	resellerPlanSvc *service.ResellerPlanService,
	resellerCodeRepo service.ResellerCodeRepository,
) *ResellerCodeHandler {
	return &ResellerCodeHandler{
		redeemService:    redeemService,
		resellerPlanSvc:  resellerPlanSvc,
		resellerCodeRepo: resellerCodeRepo,
	}
}

type generateResellerCodesRequest struct {
	Count   int     `json:"count" binding:"required"`
	Value   float64 `json:"value" binding:"required"`
	GroupID *int64  `json:"group_id"`
	Notes   string  `json:"notes"`
}

// GenerateCodes issues redeem codes against the caller's balance.
// POST /api/v1/reseller/codes
//
// The user id comes from the authenticated session, never from the body: a
// reseller must not be able to mint codes billed to somebody else's balance.
func (h *ResellerCodeHandler) GenerateCodes(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	userID := subject.UserID

	var req generateResellerCodesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	codes, err := h.redeemService.GenerateResellerCodes(
		c.Request.Context(),
		h.resellerPlanSvc,
		h.resellerCodeRepo,
		service.ResellerCodeRequest{
			UserID:  userID,
			Count:   req.Count,
			Value:   req.Value,
			GroupID: req.GroupID,
			Notes:   req.Notes,
		},
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// The full code strings are returned once, here. A reseller has to see them
	// to sell them, so there is no masking to be had — which is exactly why the
	// account holding them warrants 2FA.
	items := make([]gin.H, 0, len(codes))
	for _, code := range codes {
		items = append(items, gin.H{"code": code.Code, "value": code.Value})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"codes": items, "count": len(items)}})
}
