package repository

import (
	"context"
	"fmt"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type resellerCodeRepository struct {
	client *dbent.Client
}

func NewResellerCodeRepository(client *dbent.Client) service.ResellerCodeRepository {
	return &resellerCodeRepository{client: client}
}

// GenerateForReseller debits the reseller and inserts their codes atomically.
//
// The debit is a guarded update with no unconditional fallback, unlike
// UserRepository.DeductBalance. That one may take a balance negative, which is
// correct for usage billing — a request already streaming cannot be refused —
// and wrong here: a redeem code is a bearer instrument, and minting more of
// them than the reseller has paid for creates money.
//
// Both halves share one transaction because the failure modes of splitting
// them are ugly in opposite directions: debit-then-crash silently burns the
// reseller's balance, insert-then-crash hands out codes nobody paid for.
func (r *resellerCodeRepository) GenerateForReseller(
	ctx context.Context,
	userID int64,
	totalCost float64,
	codes []service.RedeemCode,
) error {
	if len(codes) == 0 {
		return nil
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin reseller code generation: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	// BalanceGTE inside the UPDATE, not a SELECT beforehand: two concurrent
	// requests that both read "enough" and then both write would each pass a
	// pre-check and together overdraw.
	affected, err := tx.User.Update().
		Where(dbuser.IDEQ(userID), dbuser.BalanceGTE(totalCost)).
		AddBalance(-totalCost).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("debit reseller %d: %w", userID, err)
	}
	if affected == 0 {
		return infraerrors.BadRequest("INSUFFICIENT_BALANCE",
			"not enough balance to generate these codes")
	}

	builders := make([]*dbent.RedeemCodeCreate, 0, len(codes))
	for i := range codes {
		code := codes[i]
		builder := tx.RedeemCode.Create().
			SetCode(code.Code).
			SetType(code.Type).
			SetValue(code.Value).
			SetStatus(code.Status).
			SetNillableCreatedBy(code.CreatedBy)
		if code.GroupID != nil {
			builder = builder.SetGroupID(*code.GroupID)
		}
		if code.Notes != "" {
			builder = builder.SetNotes(code.Notes)
		}
		builders = append(builders, builder)
	}

	if err := tx.RedeemCode.CreateBulk(builders...).Exec(ctx); err != nil {
		return fmt.Errorf("insert reseller codes: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reseller code generation: %w", err)
	}
	tx = nil
	return nil
}
