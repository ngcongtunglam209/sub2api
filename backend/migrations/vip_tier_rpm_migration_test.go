//go:build unit

package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration230AddsVIPRPMAndExemptionColumns(t *testing.T) {
	content, err := FS.ReadFile("230_vip_tier_rpm.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "ALTER TABLE vip_tiers")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS rpm_limit INTEGER NOT NULL DEFAULT 0")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS unlimited_rpm BOOLEAN NOT NULL DEFAULT FALSE")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS unlimited_concurrency BOOLEAN NOT NULL DEFAULT FALSE")
}

// The defaults have to leave every existing tier exactly as it was: rpm_limit 0
// grants no RPM, and both exemptions off. A backfill or a non-zero default would
// silently hand out a perk on the day this migration ran — and in the case of
// the exemptions, would uncap the fleet's scarcest resource.
func TestMigration230ChangesNoExistingTierBehaviour(t *testing.T) {
	content, err := FS.ReadFile("230_vip_tier_rpm.sql")
	require.NoError(t, err)

	sql := string(content)
	require.NotContains(t, sql, "UPDATE vip_tiers")
	require.NotContains(t, sql, "DEFAULT TRUE")
}
