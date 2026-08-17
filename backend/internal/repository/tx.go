package repository

import (
	"context"
	"fmt"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

// runInTx runs fn inside a transaction, joining one already carried on the
// context instead of nesting a second.
//
// Nesting is the case worth spelling out. `client.Tx(ctx)` always opens a
// fresh connection-level transaction, so a repository method that calls it
// unconditionally cannot be composed: a caller who debits a balance and then
// invokes that method ends up with two transactions that commit independently,
// which is precisely the debit-then-crash hole the transaction was for. When
// the context already carries a transaction, fn joins it and commit and
// rollback stay with whoever opened it — one owner, one outcome.
//
// fn receives a context bound to the transaction (so nested repository calls
// join it too) and the transaction's client.
func runInTx(
	ctx context.Context,
	client *dbent.Client,
	what string,
	fn func(ctx context.Context, client *dbent.Client) error,
) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return fn(ctx, tx.Client())
	}

	tx, err := client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin %s: %w", what, err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	txCtx := dbent.NewTxContext(ctx, tx)
	if err := fn(txCtx, tx.Client()); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit %s: %w", what, err)
	}
	committed = true
	return nil
}
