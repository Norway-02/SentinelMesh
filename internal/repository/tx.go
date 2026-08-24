package repository

import (
	"context"
)

// Tx represents a generic transaction handle
type Tx interface {
	// Commit commits the transaction
	Commit() error
	// Rollback aborts the transaction
	Rollback() error
}

// TxManager encapsulates business transactions
type TxManager interface {
	// WithinTx executes the provided function within a transactional boundary.
	// If the function returns an error, the transaction is rolled back.
	// Otherwise, it is committed.
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
