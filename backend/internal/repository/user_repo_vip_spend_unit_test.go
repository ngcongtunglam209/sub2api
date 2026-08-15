package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

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

// The sweep is what makes an expired tier have to be re-earned. Getting the
// filters wrong either strands users on a tier forever or wipes the spend of
// customers who are still paying.
func TestExpireVIPTiers(t *testing.T) {
	ctx := context.Background()
	repo, client := newVIPSpendRepoSQLite(t)
	tierID := mustCreateVIPTier(t, ctx, client, 5, 0.8, 16)

	newGraded := func(email string, expiresAt time.Time, locked bool) int64 {
		id := mustCreateVIPSpendUser(t, ctx, client, email)
		require.NoError(t, client.User.UpdateOneID(id).
			SetVipTierID(tierID).
			SetVipExpiresAt(expiresAt).
			SetVipTierLocked(locked).
			SetVipQualifyingSpend(500).
			SetTotalPaidUsd(500).
			Exec(ctx))
		return id
	}

	lapsed := newGraded("vip-sweep-lapsed@example.com", time.Now().Add(-time.Hour), false)
	active := newGraded("vip-sweep-active@example.com", time.Now().Add(time.Hour), false)
	locked := newGraded("vip-sweep-locked@example.com", time.Now().Add(-time.Hour), true)

	ids, err := repo.ListExpiredVIPUserIDs(ctx, time.Now(), 100)
	require.NoError(t, err)
	require.Equal(t, []int64{lapsed}, ids, "only lapsed, unlocked tiers may be swept")

	n, err := repo.ExpireVIPTiers(ctx, ids)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	cleared, err := client.User.Get(ctx, lapsed)
	require.NoError(t, err)
	require.Nil(t, cleared.VipTierID)
	require.Nil(t, cleared.VipExpiresAt)
	require.InDelta(t, 0, cleared.VipQualifyingSpend, 1e-9)
	// The lifetime figure is what reports read; only the grading counter resets.
	require.InDelta(t, 500, cleared.TotalPaidUsd, 1e-9)

	for _, id := range []int64{active, locked} {
		kept, err := client.User.Get(ctx, id)
		require.NoError(t, err)
		require.NotNil(t, kept.VipTierID)
		require.InDelta(t, 500, kept.VipQualifyingSpend, 1e-9)
	}
}

// A locked id slipping into the batch (raced with an admin lock) must still be
// filtered by the UPDATE itself, not just by the SELECT that produced it.
func TestExpireVIPTiers_SkipsLockedEvenWhenAsked(t *testing.T) {
	ctx := context.Background()
	repo, client := newVIPSpendRepoSQLite(t)
	tierID := mustCreateVIPTier(t, ctx, client, 6, 0.8, 16)

	id := mustCreateVIPSpendUser(t, ctx, client, "vip-sweep-race@example.com")
	require.NoError(t, client.User.UpdateOneID(id).
		SetVipTierID(tierID).
		SetVipExpiresAt(time.Now().Add(-time.Hour)).
		SetVipTierLocked(true).
		Exec(ctx))

	n, err := repo.ExpireVIPTiers(ctx, []int64{id})
	require.NoError(t, err)
	require.Zero(t, n)

	kept, err := client.User.Get(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, kept.VipTierID)
}

func TestExpireVIPTiers_EmptyInput(t *testing.T) {
	ctx := context.Background()
	repo, _ := newVIPSpendRepoSQLite(t)

	n, err := repo.ExpireVIPTiers(ctx, nil)
	require.NoError(t, err)
	require.Zero(t, n)
}

func mustCreateVIPTier(t *testing.T, ctx context.Context, client *dbent.Client, level int, rate float64, concurrency int) int64 {
	t.Helper()
	tier, err := client.VIPTier.Create().
		SetLevel(level).
		SetName("VIP").
		SetMinSpendUsd(20).
		SetRateMultiplier(rate).
		SetConcurrency(concurrency).
		Save(ctx)
	require.NoError(t, err)
	return tier.ID
}

// The perks read here gate real money and real capacity, so every way a tier
// can be absent has to land on "no discount, no extra slots".
func TestActiveVIPTierBenefits(t *testing.T) {
	ctx := context.Background()
	repo, client := newVIPSpendRepoSQLite(t)
	tierID := mustCreateVIPTier(t, ctx, client, 2, 0.9, 12)

	t.Run("no tier", func(t *testing.T) {
		id := mustCreateVIPSpendUser(t, ctx, client, "vip-none@example.com")
		rate, err := repo.GetVIPRateMultiplier(ctx, id)
		require.NoError(t, err)
		require.InDelta(t, 1, rate, 1e-9)
		concurrency, err := repo.GetVIPConcurrency(ctx, id)
		require.NoError(t, err)
		require.Zero(t, concurrency)
	})

	t.Run("active tier", func(t *testing.T) {
		id := mustCreateVIPSpendUser(t, ctx, client, "vip-active@example.com")
		require.NoError(t, client.User.UpdateOneID(id).
			SetVipTierID(tierID).
			SetVipExpiresAt(time.Now().Add(24*time.Hour)).
			Exec(ctx))

		rate, err := repo.GetVIPRateMultiplier(ctx, id)
		require.NoError(t, err)
		require.InDelta(t, 0.9, rate, 1e-9)
		concurrency, err := repo.GetVIPConcurrency(ctx, id)
		require.NoError(t, err)
		require.Equal(t, 12, concurrency)
	})

	// Read time, not sweep time, decides: a stalled expiry job must not keep
	// handing out a discount forever.
	t.Run("lapsed tier", func(t *testing.T) {
		id := mustCreateVIPSpendUser(t, ctx, client, "vip-lapsed@example.com")
		require.NoError(t, client.User.UpdateOneID(id).
			SetVipTierID(tierID).
			SetVipExpiresAt(time.Now().Add(-time.Hour)).
			Exec(ctx))

		rate, err := repo.GetVIPRateMultiplier(ctx, id)
		require.NoError(t, err)
		require.InDelta(t, 1, rate, 1e-9)
	})

	t.Run("locked tier ignores expiry", func(t *testing.T) {
		id := mustCreateVIPSpendUser(t, ctx, client, "vip-locked@example.com")
		require.NoError(t, client.User.UpdateOneID(id).
			SetVipTierID(tierID).
			SetVipExpiresAt(time.Now().Add(-time.Hour)).
			SetVipTierLocked(true).
			Exec(ctx))

		rate, err := repo.GetVIPRateMultiplier(ctx, id)
		require.NoError(t, err)
		require.InDelta(t, 0.9, rate, 1e-9)
	})

	t.Run("tier row deleted", func(t *testing.T) {
		id := mustCreateVIPSpendUser(t, ctx, client, "vip-dangling@example.com")
		require.NoError(t, client.User.UpdateOneID(id).
			SetVipTierID(999999).
			SetVipExpiresAt(time.Now().Add(24*time.Hour)).
			Exec(ctx))

		rate, err := repo.GetVIPRateMultiplier(ctx, id)
		require.NoError(t, err)
		require.InDelta(t, 1, rate, 1e-9)
	})
}
