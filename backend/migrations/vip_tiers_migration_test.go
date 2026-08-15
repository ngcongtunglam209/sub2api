//go:build unit

package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration224CreatesVIPTiers(t *testing.T) {
	content, err := FS.ReadFile("224_vip_tiers.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS vip_tiers")
	require.Contains(t, sql, "level INTEGER NOT NULL UNIQUE")
	require.Contains(t, sql, "min_spend_usd DECIMAL(20, 2) NOT NULL")
	require.Contains(t, sql, "rate_multiplier DECIMAL(6, 4) NOT NULL DEFAULT 1")
	require.Contains(t, sql, "grace_days INTEGER NOT NULL DEFAULT 60")
	require.Contains(t, sql, "enabled BOOLEAN NOT NULL DEFAULT TRUE")
	// Re-running the migration must not resurrect tiers an operator deleted or
	// overwrite thresholds they retuned in the admin UI.
	require.Contains(t, sql, "ON CONFLICT (level) DO NOTHING")
}

func TestMigration224SeedsTheAgreedLadder(t *testing.T) {
	content, err := FS.ReadFile("224_vip_tiers.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "(1, 'VIP1', 20, 0.95, 8, 60,")
	require.Contains(t, sql, "(2, 'VIP2', 100, 0.90, 12, 60,")
	require.Contains(t, sql, "(3, 'VIP3', 400, 0.82, 20, 60,")
	require.Contains(t, sql, "(4, 'VIP4', 1500, 0.70, 32, 60,")
}

func TestMigration224AddsUserVIPColumns(t *testing.T) {
	content, err := FS.ReadFile("224_vip_tiers.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS total_paid_usd DECIMAL(20, 8) NOT NULL DEFAULT 0")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS vip_qualifying_spend DECIMAL(20, 8) NOT NULL DEFAULT 0")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS vip_tier_id BIGINT")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS vip_expires_at TIMESTAMPTZ")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS vip_tier_locked BOOLEAN NOT NULL DEFAULT FALSE")
	// The expiry sweep reads vip_expires_at for graded users only; a partial
	// index keeps it off every row of a table where most users have no tier.
	require.Contains(t, sql, "CREATE INDEX IF NOT EXISTS idx_users_vip_expires_at")
	require.Contains(t, sql, "WHERE vip_tier_id IS NOT NULL")
}
