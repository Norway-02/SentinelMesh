package outbox

import (
	"context"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/events"
)

// Repository defines the interface for outbox persistence
type Repository interface {
	Insert(ctx context.Context, event events.Event) error
	Claim(ctx context.Context, batchSize int, ownerID string, claimDuration time.Duration) ([]events.Event, error)
	MarkPublished(ctx context.Context, eventID string) error
	MarkFailed(ctx context.Context, eventID string, errStr string) error
}

type txKey struct{} // Must match the one in postgres package, or we export a shared one. Wait, we can just use an exported TxManager interface, but `extractDB` needs to work. Let's create a shared package for DB context, or just export the key.

// For now, let's just make postgres.AgentRepository and this one use a shared way, or put the Postgres implementation in the postgres package.
