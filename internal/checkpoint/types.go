package checkpoint

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	ErrCorruptedCheckpoint = errors.New("checkpoint: checksum verification failed / state corrupted")
	ErrSequenceConflict    = errors.New("checkpoint: sequence conflict")
	ErrNonMonotonicSeq     = errors.New("checkpoint: sequence number must be monotonically greater than latest")
	ErrCheckpointNotFound  = errors.New("checkpoint: checkpoint not found")
	ErrInvalidCheckpoint   = errors.New("checkpoint: invalid checkpoint data")
)

// Checkpoint represents an immutable, application-level snapshot of agent state.
type Checkpoint struct {
	ID             string            `json:"id"`
	RunID          string            `json:"run_id"`
	AgentID        string            `json:"agent_id"`
	TenantID       string            `json:"tenant_id"`
	SequenceNumber int64             `json:"sequence_number"`
	StateInline    json.RawMessage   `json:"state_inline,omitempty"` // For inline JSON state
	StateURI       string            `json:"state_uri,omitempty"`    // For large object storage blobs
	StateChecksum  string            `json:"state_checksum"`         // SHA-256 over canonical byte payload
	SizeBytes      int64             `json:"size_bytes"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
}

// ComputeCanonicalChecksum computes the deterministic SHA-256 checksum over state data.
func ComputeCanonicalChecksum(stateData []byte) string {
	if len(stateData) == 0 {
		return ""
	}
	hash := sha256.Sum256(stateData)
	return hex.EncodeToString(hash[:])
}

// Validate checks basic checkpoint fields and computes checksum if missing.
func (c *Checkpoint) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("%w: missing id", ErrInvalidCheckpoint)
	}
	if c.RunID == "" {
		return fmt.Errorf("%w: missing run_id", ErrInvalidCheckpoint)
	}
	if c.AgentID == "" {
		return fmt.Errorf("%w: missing agent_id", ErrInvalidCheckpoint)
	}
	if c.SequenceNumber <= 0 {
		return fmt.Errorf("%w: sequence_number must be positive", ErrInvalidCheckpoint)
	}
	if len(c.StateInline) == 0 && c.StateURI == "" {
		return fmt.Errorf("%w: either state_inline or state_uri must be provided", ErrInvalidCheckpoint)
	}

	// Compute / verify checksum
	if len(c.StateInline) > 0 {
		expectedChecksum := ComputeCanonicalChecksum(c.StateInline)
		if c.StateChecksum == "" {
			c.StateChecksum = expectedChecksum
		} else if c.StateChecksum != expectedChecksum {
			return ErrCorruptedCheckpoint
		}
		c.SizeBytes = int64(len(c.StateInline))
	}

	return nil
}

// VerifyIntegrity confirms that the checkpoint's stored state matches its checksum.
func (c *Checkpoint) VerifyIntegrity() bool {
	if len(c.StateInline) > 0 {
		return ComputeCanonicalChecksum(c.StateInline) == c.StateChecksum
	}
	// For URI-based state, checksum verification happens when reading blob
	return c.StateChecksum != ""
}
