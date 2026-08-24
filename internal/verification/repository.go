package verification

import "context"

// Repository defines persistence operations for attestation records.
type Repository interface {
	Save(ctx context.Context, record AttestationRecord) error
	GetByID(ctx context.Context, id string) (*AttestationRecord, error)
	GetByRunID(ctx context.Context, runID string) (*AttestationRecord, error)
}
