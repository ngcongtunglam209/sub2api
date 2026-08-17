package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
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

// GetPlan returns the caller's own reseller plan, or null when they hold none.
// GET /api/v1/reseller/plan
//
// An expired plan is still reported, with active=false: a reseller whose tier
// lapsed needs to be told that rather than shown the same empty answer as
// somebody who never bought one.
func (h *ResellerCodeHandler) GetPlan(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}

	assignment, err := h.resellerPlanSvc.ResolveForUser(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// dto.ResellerPlanAssignmentFromService returns nil for "holds nothing",
	// which serialises as null — the shape the endpoint promises.
	response.Success(c, dto.ResellerPlanAssignmentFromService(assignment))
}

// ListCodes returns the codes the caller minted, paginated.
// GET /api/v1/reseller/codes
//
// The user id comes from the session and goes straight into a created_by
// filter in the query. There is no user parameter to tamper with, and no
// unscoped path through the repository method it calls: another reseller's
// unsold codes are money, and reading them is taking it.
func (h *ResellerCodeHandler) ListCodes(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}

	page, pageSize := response.ParsePagination(c)
	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "created_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}

	codes, result, err := h.redeemService.ListResellerCodes(c.Request.Context(), subject.UserID, params)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.ResellerCode, 0, len(codes))
	for i := range codes {
		if mapped := dto.ResellerCodeFromService(&codes[i]); mapped != nil {
			out = append(out, *mapped)
		}
	}
	response.Paginated(c, out, result.Total, page, pageSize)
}
