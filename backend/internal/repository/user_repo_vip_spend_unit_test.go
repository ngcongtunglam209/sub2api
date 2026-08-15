package repository

import (
	"context"
	"database/sql"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func newVIPSpendRepoSQLite(t *testing.T) (*userRepository, *dbent.Client) {
	t.Helper()

	db, err := sql.Open("sqlite", "file:user_repo_vip_spend?mode=memory&cache=shared&_fk=1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	return &userRepository{client: client, sql: db}, client
}

func mustCreateVIPSpendUser(t *testing.T, ctx context.Context, client *dbent.Client, email string) int64 {
	t.Helper()
	u, err := client.User.Create().
		SetEmail(email).
		SetPasswordHash("hash").
		Save(ctx)
	require.NoError(t, err)
	return u.ID
}

// AddVIPSpend feeds the two counters VIP grading reads. They are separate from
// total_recharged on purpose, so this asserts the recharge total stays put:
// crediting it here would let promo bonuses and affiliate withdrawals, which
// also write total_recharged, drag the tier up.
func TestAddVIPSpend_AccumulatesBothCountersOnly(t *testing.T) {
	ctx := context.Background()
	repo, client := newVIPSpendRepoSQLite(t)
	id := mustCreateVIPSpendUser(t, ctx, client, "vip-spend-accumulate@example.com")

	require.NoError(t, repo.AddVIPSpend(ctx, id, 20))
	require.NoError(t, repo.AddVIPSpend(ctx, id, 5.5))

	got, err := client.User.Get(ctx, id)
	require.NoError(t, err)
	require.InDelta(t, 25.5, got.TotalPaidUsd, 1e-9)
	require.InDelta(t, 25.5, got.VipQualifyingSpend, 1e-9)
	require.InDelta(t, 0, got.TotalRecharged, 1e-9)
}

// Refunds and zero-value orders must not walk a user backwards down the ladder:
// tiers are earned, and an amount that is not a positive charge is a no-op.
func TestAddVIPSpend_IgnoresNonPositiveAmounts(t *testing.T) {
	ctx := context.Background()
	repo, client := newVIPSpendRepoSQLite(t)
	id := mustCreateVIPSpendUser(t, ctx, client, "vip-spend-nonpositive@example.com")

	require.NoError(t, repo.AddVIPSpend(ctx, id, 40))
	require.NoError(t, repo.AddVIPSpend(ctx, id, 0))
	require.NoError(t, repo.AddVIPSpend(ctx, id, -10))

	got, err := client.User.Get(ctx, id)
	require.NoError(t, err)
	require.InDelta(t, 40, got.TotalPaidUsd, 1e-9)
	require.InDelta(t, 40, got.VipQualifyingSpend, 1e-9)
}

func TestAddVIPSpend_UnknownUser(t *testing.T) {
	ctx := context.Background()
	repo, _ := newVIPSpendRepoSQLite(t)

	err := repo.AddVIPSpend(ctx, 987654, 10)
	require.ErrorIs(t, err, service.ErrUserNotFound)
}
