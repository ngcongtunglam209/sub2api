package repository

import (
	"context"
	"database/sql"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

// newRedeemCodeCreatorScopeRepo builds an isolated in-memory schema.
//
// The database name is per-test: `cache=shared` keeps one in-memory database
// alive under a given name, so reusing it would let codes minted by one case
// show up in the next and quietly weaken exactly the assertion these tests make.
func newRedeemCodeCreatorScopeRepo(t *testing.T) (*redeemCodeRepository, *dbent.Client) {
	t.Helper()

	db, err := sql.Open("sqlite", "file:redeem_code_creator_scope_"+t.Name()+"?mode=memory&cache=shared&_fk=1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	return &redeemCodeRepository{client: client}, client
}

func mustCreateCodeWithCreator(t *testing.T, ctx context.Context, client *dbent.Client, code string, creator *int64) {
	t.Helper()
	err := client.RedeemCode.Create().
		SetCode(code).
		SetType(service.RedeemTypeBalance).
		SetValue(10).
		SetStatus(service.StatusUnused).
		SetNillableCreatedBy(creator).
		Exec(ctx)
	require.NoError(t, err)
}

// Unsold redeem codes are bearer instruments: whoever reads the string can
// spend it. So the scoping is asserted against the query, not the handler —
// one reseller seeing another's stock is theft, and a handler-level check is
// one refactor away from being skipped.
func TestListByCreatorReturnsOnlyTheCallersCodes(t *testing.T) {
	ctx := context.Background()
	repo, client := newRedeemCodeCreatorScopeRepo(t)

	mine, theirs := int64(11), int64(22)
	mustCreateCodeWithCreator(t, ctx, client, "mine-a", &mine)
	mustCreateCodeWithCreator(t, ctx, client, "mine-b", &mine)
	mustCreateCodeWithCreator(t, ctx, client, "theirs-a", &theirs)
	// created_by NULL is a platform/admin-issued code. It belongs to nobody and
	// must not fall into any reseller's listing.
	mustCreateCodeWithCreator(t, ctx, client, "platform-a", nil)

	params := pagination.PaginationParams{Page: 1, PageSize: 50}

	codes, result, err := repo.ListByCreator(ctx, mine, params)
	require.NoError(t, err)
	require.EqualValues(t, 2, result.Total, "total must count only the caller's codes, not every row")

	got := make([]string, 0, len(codes))
	for _, code := range codes {
		got = append(got, code.Code)
		require.NotNil(t, code.CreatedBy)
		require.Equal(t, mine, *code.CreatedBy)
	}
	require.ElementsMatch(t, []string{"mine-a", "mine-b"}, got)
}

// A reseller who has minted nothing gets an empty page, never the unscoped
// list that a missing WHERE clause would produce.
func TestListByCreatorIsEmptyForANonReseller(t *testing.T) {
	ctx := context.Background()
	repo, client := newRedeemCodeCreatorScopeRepo(t)

	other := int64(33)
	mustCreateCodeWithCreator(t, ctx, client, "other-a", &other)
	mustCreateCodeWithCreator(t, ctx, client, "platform-b", nil)

	codes, result, err := repo.ListByCreator(ctx, 999, pagination.PaginationParams{Page: 1, PageSize: 50})
	require.NoError(t, err)
	require.Empty(t, codes)
	require.EqualValues(t, 0, result.Total)
}

// Paging must not be a way out of the scope: page 2 of somebody with one page
// of codes is empty, not the next slice of everyone else's.
func TestListByCreatorScopesEveryPage(t *testing.T) {
	ctx := context.Background()
	repo, client := newRedeemCodeCreatorScopeRepo(t)

	mine, theirs := int64(44), int64(55)
	mustCreateCodeWithCreator(t, ctx, client, "p-mine-a", &mine)
	mustCreateCodeWithCreator(t, ctx, client, "p-mine-b", &mine)
	for _, code := range []string{"p-theirs-a", "p-theirs-b", "p-theirs-c"} {
		mustCreateCodeWithCreator(t, ctx, client, code, &theirs)
	}

	first, result, err := repo.ListByCreator(ctx, mine, pagination.PaginationParams{Page: 1, PageSize: 2})
	require.NoError(t, err)
	require.Len(t, first, 2)
	require.EqualValues(t, 2, result.Total)

	second, _, err := repo.ListByCreator(ctx, mine, pagination.PaginationParams{Page: 2, PageSize: 2})
	require.NoError(t, err)
	require.Empty(t, second)
}
