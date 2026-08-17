package repository

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	dbaddon "github.com/Wei-Shaw/sub2api/ent/useraddon"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type userAddonRepository struct {
	client *dbent.Client
}

func NewUserAddonRepository(client *dbent.Client) service.UserAddonRepository {
	return &userAddonRepository{client: client}
}

// ListByUser returns every add-on row a user has, expired ones included.
//
// Filtering by expiry is left to the service on purpose: one place decides
// what "still valid" means, and the catalogue endpoint can show a lapsed
// add-on as lapsed rather than as never bought.
func (r *userAddonRepository) ListByUser(ctx context.Context, userID int64) ([]*service.UserAddon, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.UserAddon.Query().
		Where(dbaddon.UserIDEQ(userID)).
		Order(dbent.Asc(dbaddon.FieldKind)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list add-ons for user %d: %w", userID, err)
	}
	out := make([]*service.UserAddon, 0, len(rows))
	for _, row := range rows {
		out = append(out, userAddonEntityToService(row))
	}
	return out, nil
}

// LockByUserKind reads one add-on FOR UPDATE.
//
// The lock is what makes the cap check hold under concurrency: without it two
// simultaneous purchases both read the same "already holds" figure and both
// conclude there is room. It only bites once a row exists — two first-ever
// purchases of the same kind have nothing to lock, and are stopped instead by
// the unique index on (user_id, kind), which fails the loser's transaction and
// takes its debit back with it.
func (r *userAddonRepository) LockByUserKind(ctx context.Context, userID int64, kind service.AddonKind) (*service.UserAddon, error) {
	client := clientFromContext(ctx, r.client)
	row, err := client.UserAddon.Query().
		Where(dbaddon.UserIDEQ(userID), dbaddon.KindEQ(string(kind))).
		ForUpdate().
		Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lock %s add-on for user %d: %w", kind, userID, err)
	}
	return userAddonEntityToService(row), nil
}

// Upsert writes the merged add-on for one (user, kind).
//
// The amount is set outright rather than added to: the service has already
// resolved the total it wants stored, having decided what of the existing row
// still counted. An AddAmount here would silently resurrect an expired
// balance the service deliberately dropped.
func (r *userAddonRepository) Upsert(
	ctx context.Context,
	userID int64,
	kind service.AddonKind,
	amount int,
	expiresAt time.Time,
) (*service.UserAddon, error) {
	client := clientFromContext(ctx, r.client)

	existing, err := client.UserAddon.Query().
		Where(dbaddon.UserIDEQ(userID), dbaddon.KindEQ(string(kind))).
		Only(ctx)
	switch {
	case err == nil:
		row, err := client.UserAddon.UpdateOneID(existing.ID).
			SetAmount(amount).
			SetExpiresAt(expiresAt).
			Save(ctx)
		if err != nil {
			return nil, fmt.Errorf("update %s add-on for user %d: %w", kind, userID, err)
		}
		return userAddonEntityToService(row), nil
	case dbent.IsNotFound(err):
		row, err := client.UserAddon.Create().
			SetUserID(userID).
			SetKind(string(kind)).
			SetAmount(amount).
			SetExpiresAt(expiresAt).
			Save(ctx)
		if err != nil {
			return nil, fmt.Errorf("create %s add-on for user %d: %w", kind, userID, err)
		}
		return userAddonEntityToService(row), nil
	default:
		return nil, fmt.Errorf("read %s add-on for user %d: %w", kind, userID, err)
	}
}

// DebitBalanceGuarded takes amount off a balance, refusing rather than
// overdrawing.
//
// BalanceGTE sits inside the UPDATE, not in a SELECT beforehand: two
// concurrent purchases that both read "enough" and then both write would each
// pass a pre-check and together overdraw. Same idiom as
// resellerCodeRepository.GenerateForReseller, and for the same reason —
// UserRepository.DeductBalance falls back to an unconditional write and can
// take a balance negative, which is right for billing work already done and
// wrong for a purchase that can simply be declined.
func (r *userAddonRepository) DebitBalanceGuarded(ctx context.Context, userID int64, amount float64) error {
	if amount <= 0 {
		return nil
	}
	client := clientFromContext(ctx, r.client)
	affected, err := client.User.Update().
		Where(dbuser.IDEQ(userID), dbuser.BalanceGTE(amount)).
		AddBalance(-amount).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("debit user %d: %w", userID, err)
	}
	if affected == 0 {
		return infraerrors.BadRequest("INSUFFICIENT_BALANCE", "not enough balance for this purchase")
	}
	return nil
}

// RunInTx exposes the transaction runner to the service layer, so the policy
// that decides *what* shares a transaction can live next to the rules it
// enforces rather than being frozen into one repository method.
func (r *userAddonRepository) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return runInTx(ctx, r.client, "add-on purchase", func(txCtx context.Context, _ *dbent.Client) error {
		return fn(txCtx)
	})
}

// ListExpiredAddonUserIDs returns users holding at least one lapsed add-on.
//
// Users rather than rows: the sweep's other job is invalidating auth
// snapshots, which are keyed by user, and a user with both kinds lapsing at
// once should be invalidated once.
func (r *userAddonRepository) ListExpiredAddonUserIDs(ctx context.Context, now time.Time, limit int) ([]int64, error) {
	if limit <= 0 {
		limit = 500
	}
	client := clientFromContext(ctx, r.client)
	rows, err := client.UserAddon.Query().
		Where(dbaddon.ExpiresAtLTE(now)).
		Order(dbent.Asc(dbaddon.FieldUserID)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list expired add-ons: %w", err)
	}

	seen := make(map[int64]struct{}, len(rows))
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		if _, ok := seen[row.UserID]; ok {
			continue
		}
		seen[row.UserID] = struct{}{}
		ids = append(ids, row.UserID)
	}
	return ids, nil
}

// DeleteExpiredAddons removes lapsed rows for the given users.
//
// The expiry is re-checked in the DELETE rather than trusted from the listing
// step: a user who renewed in between would otherwise lose the add-on they
// just paid for, and the listing is deliberately not held under a lock.
func (r *userAddonRepository) DeleteExpiredAddons(ctx context.Context, now time.Time, userIDs []int64) (int, error) {
	if len(userIDs) == 0 {
		return 0, nil
	}
	client := clientFromContext(ctx, r.client)
	deleted, err := client.UserAddon.Delete().
		Where(
			dbaddon.UserIDIn(userIDs...),
			dbaddon.ExpiresAtLTE(now),
		).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete expired add-ons: %w", err)
	}
	return deleted, nil
}

func userAddonEntityToService(row *dbent.UserAddon) *service.UserAddon {
	if row == nil {
		return nil
	}
	return &service.UserAddon{
		ID:        row.ID,
		UserID:    row.UserID,
		Kind:      service.AddonKind(row.Kind),
		Amount:    row.Amount,
		ExpiresAt: row.ExpiresAt,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}
