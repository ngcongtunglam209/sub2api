package service

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// maxResellerCodesPerBatch bounds one generation call.
//
// Lower than the admin limit of 1000 on purpose: this path spends real balance,
// and a loop bug or a stolen session should not be able to drain an account and
// mint a thousand bearer instruments in one request.
const maxResellerCodesPerBatch = 200

// ResellerCodeRequest is one batch of codes a reseller wants to sell on.
type ResellerCodeRequest struct {
	UserID  int64
	Count   int
	Value   float64
	GroupID *int64
	Notes   string
}

// ResellerCodeRepository issues codes against a reseller's balance.
type ResellerCodeRepository interface {
	// GenerateForReseller debits the reseller and inserts the codes in one
	// transaction, refusing rather than overdrawing.
	//
	// Deliberately not reusing UserRepository.DeductBalance: that one falls
	// back to an unconditional write when the guarded update matches nothing,
	// so it can take a balance negative. That is the right call for usage
	// billing, which cannot refuse a request already in flight, and the wrong
	// one here — a reseller must never mint codes worth more than they hold.
	GenerateForReseller(ctx context.Context, userID int64, totalCost float64, codes []RedeemCode) error
}

// GenerateResellerCodes turns a reseller's balance into redeem codes they can
// sell on at whatever price their market bears.
//
// The face value is debited at par: there is no discount here. A reseller's
// margin is the spread between what they charge for a code and its face value,
// which is entirely theirs and never passes through this system. The tier
// discount was already paid out once, as the credit that came with the plan.
func (s *RedeemService) GenerateResellerCodes(
	ctx context.Context,
	planResolver ResellerPlanResolver,
	repo ResellerCodeRepository,
	req ResellerCodeRequest,
) ([]RedeemCode, error) {
	if req.UserID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "a reseller is required")
	}
	if req.Count <= 0 || req.Count > maxResellerCodesPerBatch {
		return nil, infraerrors.BadRequest("INVALID_CODE_COUNT",
			fmt.Sprintf("count must be between 1 and %d", maxResellerCodesPerBatch))
	}
	if req.Value <= 0 {
		return nil, infraerrors.BadRequest("INVALID_CODE_VALUE", "code value must be greater than zero")
	}

	assignment, err := planResolver.ResolveForUser(ctx, req.UserID)
	if err != nil {
		return nil, err
	}
	if !assignment.Active(time.Now()) {
		return nil, infraerrors.Forbidden("RESELLER_PLAN_REQUIRED", "an active reseller plan is required to generate codes")
	}
	if err := checkResellerGroupAllowed(assignment.Plan, req.GroupID); err != nil {
		return nil, err
	}

	// decimal, not float64: count × value is money, and 3 × 0.1 in binary
	// floating point is not 0.3.
	totalCost, _ := decimal.NewFromInt(int64(req.Count)).
		Mul(decimal.NewFromFloat(req.Value)).
		Round(2).
		Float64()

	codes := make([]RedeemCode, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		code, err := s.GenerateRandomCode()
		if err != nil {
			return nil, fmt.Errorf("generate code: %w", err)
		}
		entry := RedeemCode{
			Code:      code,
			Type:      RedeemTypeBalance,
			Value:     req.Value,
			Status:    StatusUnused,
			CreatedBy: &req.UserID,
		}
		if req.GroupID != nil {
			entry.GroupID = req.GroupID
		}
		entry.Notes = req.Notes
		codes = append(codes, entry)
	}

	if err := repo.GenerateForReseller(ctx, req.UserID, totalCost, codes); err != nil {
		return nil, err
	}
	return codes, nil
}

// ResellerPlanResolver is the slice of the plan service this path needs.
type ResellerPlanResolver interface {
	ResolveForUser(ctx context.Context, userID int64) (*ResellerPlanAssignment, error)
}

// checkResellerGroupAllowed keeps a tier to the groups it was sold.
//
// An empty whitelist means unrestricted. Resellers pick from groups we define
// rather than setting rates themselves — a rate they controlled would only hand
// their own customers fewer tokens and earn them nothing.
func checkResellerGroupAllowed(plan *ResellerPlan, groupID *int64) error {
	if groupID == nil || len(plan.AllowedGroupIDs) == 0 {
		return nil
	}
	for _, allowed := range plan.AllowedGroupIDs {
		if allowed == *groupID {
			return nil
		}
	}
	return infraerrors.Forbidden("RESELLER_GROUP_NOT_ALLOWED", "this reseller plan cannot sell codes for that group")
}
