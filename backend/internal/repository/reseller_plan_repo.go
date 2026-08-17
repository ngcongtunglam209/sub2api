package repository

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/resellerplan"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type resellerPlanRepository struct {
	client *dbent.Client
}

func NewResellerPlanRepository(client *dbent.Client) service.ResellerPlanRepository {
	return &resellerPlanRepository{client: client}
}

func (r *resellerPlanRepository) List(ctx context.Context) ([]*service.ResellerPlan, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.ResellerPlan.Query().
		Order(dbent.Asc(resellerplan.FieldLevel)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list reseller plans: %w", err)
	}
	result := make([]*service.ResellerPlan, 0, len(rows))
	for _, row := range rows {
		result = append(result, resellerPlanEntityToService(row))
	}
	return result, nil
}

func (r *resellerPlanRepository) GetByID(ctx context.Context, id int64) (*service.ResellerPlan, error) {
	client := clientFromContext(ctx, r.client)
	row, err := client.ResellerPlan.Get(ctx, id)
	if dbent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get reseller plan %d: %w", id, err)
	}
	return resellerPlanEntityToService(row), nil
}

// AssignToUser stamps the plan and credits the balance in one transaction.
//
// The two writes cannot be split. A crash between them either leaves a
// reseller holding a tier nobody paid for, or takes the payment and never
// hands back the credit — both need a human to work out which.
func (r *resellerPlanRepository) AssignToUser(
	ctx context.Context,
	userID int64,
	plan *service.ResellerPlan,
	expiresAt time.Time,
	creditAmount float64,
) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin reseller plan assignment: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	update := tx.User.UpdateOneID(userID).
		SetResellerPlanID(plan.ID).
		SetResellerPlanExpiresAt(expiresAt)

	// RPM is an override, so it is safe to set outright. Concurrency is not
	// touched here: it is additive and resolved on the auth snapshot, and
	// writing a total into users.concurrency would lose the base value the
	// bonus is supposed to sit on top of.
	if plan.RPMLimit > 0 {
		update = update.SetRpmLimit(plan.RPMLimit)
	}
	if creditAmount > 0 {
		update = update.AddBalance(creditAmount)
	}

	if err := update.Exec(ctx); err != nil {
		return fmt.Errorf("assign reseller plan %d to user %d: %w", plan.ID, userID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reseller plan assignment: %w", err)
	}
	tx = nil
	return nil
}

func (r *resellerPlanRepository) GetUserAssignment(ctx context.Context, userID int64) (*service.ResellerPlanAssignment, error) {
	client := clientFromContext(ctx, r.client)
	row, err := client.User.Query().
		Where(user.IDEQ(userID)).
		Select(user.FieldResellerPlanID, user.FieldResellerPlanExpiresAt).
		Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get reseller assignment for user %d: %w", userID, err)
	}
	if row.ResellerPlanID == nil || row.ResellerPlanExpiresAt == nil {
		return nil, nil
	}

	plan, err := r.GetByID(ctx, *row.ResellerPlanID)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		// The plan row was deleted out from under the user. Treat them as
		// holding nothing rather than erroring: a deleted tier is an operator
		// action, not a broken account.
		return nil, nil
	}

	return &service.ResellerPlanAssignment{Plan: plan, ExpiresAt: *row.ResellerPlanExpiresAt}, nil
}

func (r *resellerPlanRepository) ClearUserAssignment(ctx context.Context, userID int64) error {
	client := clientFromContext(ctx, r.client)
	err := client.User.UpdateOneID(userID).
		ClearResellerPlanID().
		ClearResellerPlanExpiresAt().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("clear reseller assignment for user %d: %w", userID, err)
	}
	return nil
}

func resellerPlanEntityToService(row *dbent.ResellerPlan) *service.ResellerPlan {
	if row == nil {
		return nil
	}
	return &service.ResellerPlan{
		ID:               row.ID,
		Level:            row.Level,
		Name:             row.Name,
		Price:            row.Price,
		CreditRate:       row.CreditRate,
		ConcurrencyBonus: row.ConcurrencyBonus,
		RPMLimit:         row.RpmLimit,
		MaxDomains:       row.MaxDomains,
		ValidityDays:     row.ValidityDays,
		AllowedGroupIDs:  row.AllowedGroupIds,
		Enabled:          row.Enabled,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
}
