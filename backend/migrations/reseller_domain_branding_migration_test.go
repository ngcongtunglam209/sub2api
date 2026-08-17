//go:build unit

package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration229AddsNullableBrandingColumns(t *testing.T) {
	content, err := FS.ReadFile("229_reseller_domain_branding.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "ALTER TABLE reseller_domains")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS site_name VARCHAR(100)")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS site_logo TEXT")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS site_subtitle VARCHAR(200)")
}

// The columns must be nullable and unbackfilled: NULL means "use the global
// setting", so a default or a backfill would turn every existing domain into a
// configured override on the day this migration ran.
func TestMigration229LeavesExistingDomainsOnTheGlobalBranding(t *testing.T) {
	content, err := FS.ReadFile("229_reseller_domain_branding.sql")
	require.NoError(t, err)

	sql := string(content)
	require.NotContains(t, sql, "NOT NULL")
	require.NotContains(t, sql, "DEFAULT")
	require.NotContains(t, sql, "UPDATE reseller_domains")
}
