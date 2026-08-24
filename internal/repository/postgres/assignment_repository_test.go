package postgres_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/postgres"
	"github.com/sentinelmesh/sentinelmesh/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConcurrentAssignment(t *testing.T) {
	db := testutil.SetupTestDB(t)

	repo := postgres.NewAssignmentRepository(db)

	runID := uuid.New().String()

	assignmentA := &domain.SchedulingAssignment{
		RunID:            runID,
		ClusterID:        "cluster-1",
		NodeID:           "node-a",
		AlgorithmVersion: "deterministic-v1",
		Score:            0.9,
		Decision:         domain.SchedulingDecision{},
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	assignmentB := &domain.SchedulingAssignment{
		RunID:            runID,
		ClusterID:        "cluster-1",
		NodeID:           "node-b",
		AlgorithmVersion: "deterministic-v1",
		Score:            0.8,
		Decision:         domain.SchedulingDecision{},
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	var wg sync.WaitGroup
	wg.Add(2)

	var createdA, createdB bool
	var errA, errB error

	go func() {
		defer wg.Done()
		createdA, errA = repo.Assign(context.Background(), assignmentA)
	}()

	go func() {
		defer wg.Done()
		createdB, errB = repo.Assign(context.Background(), assignmentB)
	}()

	wg.Wait()

	// Exactly one should succeed in creating
	if createdA {
		assert.False(t, createdB)
		require.Error(t, errB)
		assert.Contains(t, errB.Error(), "conflict")
		assert.NoError(t, errA)
	} else if createdB {
		assert.False(t, createdA)
		require.Error(t, errA)
		assert.Contains(t, errA.Error(), "conflict")
		assert.NoError(t, errB)
	} else {
		t.Fatal("Expected at least one assignment to succeed")
	}
}
