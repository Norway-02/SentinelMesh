package memory

import (
	"context"
	"sync"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
)

type AssignmentRepository struct {
	mu          sync.RWMutex
	assignments map[string]*domain.SchedulingAssignment
}

func NewAssignmentRepository() repository.AssignmentRepository {
	return &AssignmentRepository{
		assignments: make(map[string]*domain.SchedulingAssignment),
	}
}

func (r *AssignmentRepository) Assign(ctx context.Context, assignment *domain.SchedulingAssignment) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, exists := r.assignments[assignment.RunID]; exists {
		if existing.NodeID == assignment.NodeID {
			return false, nil
		}
		return false, repository.ErrConflict
	}

	cpy := *assignment
	r.assignments[assignment.RunID] = &cpy
	return true, nil
}

func (r *AssignmentRepository) Reassign(ctx context.Context, assignment *domain.SchedulingAssignment) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cpy := *assignment
	r.assignments[assignment.RunID] = &cpy
	return nil
}
