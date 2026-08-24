package repository

import (
	"context"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
)

// AssignmentRepository handles persistence of scheduling assignments.
type AssignmentRepository interface {
	// Assign attempts to persist a scheduling assignment.
	// It returns created=true if the assignment was successfully written.
	Assign(ctx context.Context, assignment *domain.SchedulingAssignment) (created bool, err error)

	// Reassign updates the node assignment for a recovered run.
	Reassign(ctx context.Context, assignment *domain.SchedulingAssignment) error
}
