package postgres

import (
	"context"
	"database/sql"
	"fmt"
)

type txKey struct{}

// TxManager manages database transactions for Postgres
type TxManager struct {
	db *sql.DB
}

// NewTxManager creates a new Postgres TxManager
func NewTxManager(db *sql.DB) *TxManager {
	return &TxManager{db: db}
}

// WithinTx executes the function inside a transaction.
// It injects the *sql.Tx into the context.
func (tm *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	// If there's already a transaction, just reuse it (nested tx not truly supported in basic driver, but we can reuse the boundary)
	if _, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return fn(ctx)
	}

	tx, err := tm.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	txCtx := context.WithValue(ctx, txKey{}, tx)

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(txCtx); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// extractDB returns either the *sql.Tx from context, or the fallback *sql.DB
// Both *sql.Tx and *sql.DB implement the necessary ExecContext / QueryRowContext methods via an interface if needed,
// but since they don't share a formal interface in database/sql for everything, we return a common DB interface.
type DBExecutor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
}

func ExtractDB(ctx context.Context, defaultDB DBExecutor) DBExecutor {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return tx
	}
	return defaultDB
}

func extractDB(ctx context.Context, defaultDB DBExecutor) DBExecutor {
	return ExtractDB(ctx, defaultDB)
}
