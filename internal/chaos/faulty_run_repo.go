package chaos

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
)

// FaultyRunRepository wraps a RunRepository to inject controlled storage failures.
type FaultyRunRepository struct {
	underlying repository.RunRepository
	controller *FaultController
}

// NewFaultyRunRepository constructs a FaultyRunRepository wrapper.
func NewFaultyRunRepository(underlying repository.RunRepository, controller *FaultController) *FaultyRunRepository {
	return &FaultyRunRepository{
		underlying: underlying,
		controller: controller,
	}
}

// Create inserts a new run unless intercepted by a fault.
func (r *FaultyRunRepository) Create(ctx context.Context, run domain.AgentRun) error {
	if trigger, spec := r.controller.ShouldTrigger("Create"); trigger {
		if spec.Delay > 0 {
			time.Sleep(spec.Delay)
		}
		if spec.Type == FaultError {
			errMsg := spec.ErrorMsg
			if errMsg == "" {
				errMsg = "simulated database create error"
			}
			return errors.New(errMsg)
		}
	}
	return r.underlying.Create(ctx, run)
}

// Get fetches a run by ID unless intercepted by a fault.
func (r *FaultyRunRepository) Get(ctx context.Context, id string) (domain.AgentRun, error) {
	if trigger, spec := r.controller.ShouldTrigger("Get"); trigger {
		if spec.Delay > 0 {
			time.Sleep(spec.Delay)
		}
		if spec.Type == FaultError {
			errMsg := spec.ErrorMsg
			if errMsg == "" {
				errMsg = "simulated database read error"
			}
			return domain.AgentRun{}, errors.New(errMsg)
		}
	}
	return r.underlying.Get(ctx, id)
}

// Update updates a run unless intercepted by a fault.
func (r *FaultyRunRepository) Update(ctx context.Context, run domain.AgentRun) error {
	if trigger, spec := r.controller.ShouldTrigger("Update"); trigger {
		if spec.Delay > 0 {
			time.Sleep(spec.Delay)
		}
		if spec.Type == FaultError {
			errMsg := spec.ErrorMsg
			if errMsg == "" {
				errMsg = "simulated database update error"
			}
			return fmt.Errorf("transaction aborted: %w", errors.New(errMsg))
		}
		if spec.Type == FaultPanic {
			panic("simulated storage panic on update")
		}
	}
	return r.underlying.Update(ctx, run)
}

// ListByNode queries runs by node.
func (r *FaultyRunRepository) ListByNode(ctx context.Context, nodeID string) ([]domain.AgentRun, error) {
	if trigger, spec := r.controller.ShouldTrigger("ListByNode"); trigger {
		if spec.Delay > 0 {
			time.Sleep(spec.Delay)
		}
		if spec.Type == FaultError {
			return nil, errors.New("simulated database list error")
		}
	}
	return r.underlying.ListByNode(ctx, nodeID)
}

// ListByCluster queries runs by cluster.
func (r *FaultyRunRepository) ListByCluster(ctx context.Context, clusterID string) ([]domain.AgentRun, error) {
	if trigger, spec := r.controller.ShouldTrigger("ListByCluster"); trigger {
		if spec.Delay > 0 {
			time.Sleep(spec.Delay)
		}
		if spec.Type == FaultError {
			return nil, errors.New("simulated database list error")
		}
	}
	return r.underlying.ListByCluster(ctx, clusterID)
}
