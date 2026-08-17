package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ResellerHandler is the admin surface for reseller tiers and the custom
// domains they are entitled to.
//
// Both live on one handler because they are one operator workflow: a tier is
// sold, then the domains it allows are entered. Splitting them would mean two
// handlers, two wire entries, and two route files for what an operator does in
// a single sitting.
type ResellerHandler struct {
	planService   *service.ResellerPlanService
	domainService *service.ResellerDomainService
}

func NewResellerHandler(
	planService *service.ResellerPlanService,
	domainService *service.ResellerDomainService,
) *ResellerHandler {
	return &ResellerHandler{planService: planService, domainService: domainService}
}

// UpdateResellerPlanRequest edits a tier's terms. Every field is a pointer so
// an operator toggling `enabled` does not have to resend the price and risk
// writing back a value they read minutes ago.
type UpdateResellerPlanRequest struct {
	Price            *float64 `json:"price"`
	CreditRate       *float64 `json:"credit_rate"`
	ConcurrencyBonus *int     `json:"concurrency_bonus"`
	RPMLimit         *int     `json:"rpm_limit"`
	MaxDomains       *int     `json:"max_domains"`
	ValidityDays     *int     `json:"validity_days"`
	AllowedGroupIDs  *[]int64 `json:"allowed_group_ids"`
	Enabled          *bool    `json:"enabled"`
}

func (r UpdateResellerPlanRequest) toUpdate() service.ResellerPlanUpdate {
	return service.ResellerPlanUpdate{
		Price:            r.Price,
		CreditRate:       r.CreditRate,
		ConcurrencyBonus: r.ConcurrencyBonus,
		RPMLimit:         r.RPMLimit,
		MaxDomains:       r.MaxDomains,
		ValidityDays:     r.ValidityDays,
		AllowedGroupIDs:  r.AllowedGroupIDs,
		Enabled:          r.Enabled,
	}
}

// AssignResellerPlanRequest grants a purchased tier to a user.
type AssignResellerPlanRequest struct {
	PlanID int64 `json:"plan_id" binding:"required"`
}

// CreateResellerDomainRequest registers a hostname against a reseller.
type CreateResellerDomainRequest struct {
	Domain string `json:"domain" binding:"required"`
	UserID int64  `json:"user_id" binding:"required"`
	Notes  string `json:"notes"`
}

// UpdateResellerDomainRequest edits one domain: its status, its branding, or
// both in one request.
//
// Every field is a pointer because every field is independently optional.
// Status keeps its old meaning — omitted leaves the domain on or off as it was
// — and the three branding fields distinguish "leave it alone" (absent) from
// "clear the override" (present and empty), which is the only way an operator
// can put a domain back on the deployment's own branding.
type UpdateResellerDomainRequest struct {
	Status *string `json:"status" binding:"omitempty,oneof=active disabled"`

	SiteName     *string `json:"site_name"`
	SiteLogo     *string `json:"site_logo"`
	SiteSubtitle *string `json:"site_subtitle"`
}

func (r UpdateResellerDomainRequest) brandingUpdate() service.ResellerDomainBrandingUpdate {
	return service.ResellerDomainBrandingUpdate{
		SiteName:     r.SiteName,
		SiteLogo:     r.SiteLogo,
		SiteSubtitle: r.SiteSubtitle,
	}
}

// ListPlans handles GET /api/v1/admin/reseller-plans
func (h *ResellerHandler) ListPlans(c *gin.Context) {
	plans, err := h.planService.List(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.ResellerPlan, 0, len(plans))
	for _, plan := range plans {
		if mapped := dto.ResellerPlanFromService(plan); mapped != nil {
			out = append(out, *mapped)
		}
	}
	response.Success(c, out)
}

// UpdatePlan handles PUT /api/v1/admin/reseller-plans/:id
func (h *ResellerHandler) UpdatePlan(c *gin.Context) {
	planID, ok := parseResellerID(c, "Invalid reseller plan ID")
	if !ok {
		return
	}

	var req UpdateResellerPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	plan, err := h.planService.Update(c.Request.Context(), planID, req.toUpdate())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.ResellerPlanFromService(plan))
}

// AssignUserPlan handles POST /api/v1/admin/users/:id/reseller-plan
func (h *ResellerHandler) AssignUserPlan(c *gin.Context) {
	userID, ok := parseResellerID(c, "Invalid user ID")
	if !ok {
		return
	}

	var req AssignResellerPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	assignment, err := h.planService.AssignPlan(c.Request.Context(), userID, req.PlanID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.ResellerPlanAssignmentFromService(assignment))
}

// RevokeUserPlan handles DELETE /api/v1/admin/users/:id/reseller-plan
//
// The credited balance is deliberately not clawed back — see
// ResellerPlanService.Revoke: it left as redeem codes the moment it was
// granted, and reversing it would bounce codes already sold to third parties.
func (h *ResellerHandler) RevokeUserPlan(c *gin.Context) {
	userID, ok := parseResellerID(c, "Invalid user ID")
	if !ok {
		return
	}

	if err := h.planService.Revoke(c.Request.Context(), userID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "Reseller plan revoked successfully"})
}

// ListDomains handles GET /api/v1/admin/reseller-domains
func (h *ResellerHandler) ListDomains(c *gin.Context) {
	domains, err := h.domainService.List(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.ResellerDomain, 0, len(domains))
	for _, domain := range domains {
		if mapped := dto.ResellerDomainFromService(domain); mapped != nil {
			out = append(out, *mapped)
		}
	}
	response.Success(c, out)
}

// CreateDomain handles POST /api/v1/admin/reseller-domains
func (h *ResellerHandler) CreateDomain(c *gin.Context) {
	var req CreateResellerDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Hostname shape and the owner's domain quota are both checked in the
	// service, not here: the allowlist gates certificate issuance, so its rules
	// must hold for every caller rather than for whoever remembers to ask.
	domain, err := h.domainService.Create(c.Request.Context(), req.Domain, req.UserID, req.Notes)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.ResellerDomainFromService(domain))
}

// SetDomainStatus handles PATCH /api/v1/admin/reseller-domains/:id
//
// One endpoint for status and branding because they are one row and one
// operator action: a domain is switched on and named in the same sitting.
func (h *ResellerHandler) SetDomainStatus(c *gin.Context) {
	domainID, ok := parseResellerID(c, "Invalid reseller domain ID")
	if !ok {
		return
	}

	var req UpdateResellerDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	update := req.brandingUpdate()
	if req.Status == nil && update.IsEmpty() {
		response.BadRequest(c, "Invalid request: nothing to update")
		return
	}

	// Branding first, status second. If the branding is rejected the domain's
	// on/off state has not moved, so the operator retries one request instead
	// of discovering a half-applied edit.
	if err := h.domainService.UpdateBranding(c.Request.Context(), domainID, update); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if req.Status != nil {
		if err := h.domainService.SetStatus(c.Request.Context(), domainID, *req.Status); err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}

	response.Success(c, gin.H{"message": "Reseller domain updated successfully"})
}

// DeleteDomain handles DELETE /api/v1/admin/reseller-domains/:id
func (h *ResellerHandler) DeleteDomain(c *gin.Context) {
	domainID, ok := parseResellerID(c, "Invalid reseller domain ID")
	if !ok {
		return
	}

	if err := h.domainService.Delete(c.Request.Context(), domainID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "Reseller domain deleted successfully"})
}

func parseResellerID(c *gin.Context, message string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, message)
		return 0, false
	}
	return id, true
}
