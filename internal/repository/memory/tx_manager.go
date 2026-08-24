package memory

import (
	"context"
)

type TxManager struct{}

func NewTxManager() *TxManager {
	return &TxManager{}
}

func (tm *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	// For memory repository, we just execute it directly. Atomicity isn't strictly maintained here, 
	// but it's only for testing/local dev.
	return fn(ctx)
}
