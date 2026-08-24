package bootstrap_test

import (
	"context"
	"testing"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/bootstrap"
	"github.com/sentinelmesh/sentinelmesh/internal/config"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/postgres"
	"github.com/sentinelmesh/sentinelmesh/internal/testutil"
)

func TestDependencySelection_Memory(t *testing.T) {
	cfg := &config.Config{
		DatabaseURL: "", // Memory mode
	}

	deps, err := bootstrap.InitializeDependencies(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deps.DB != nil {
		t.Fatalf("expected DB to be nil for memory")
	}

	if _, ok := deps.AgentRepo.(*memory.AgentRepository); !ok {
		t.Fatalf("expected AgentRepo to be memory, got %T", deps.AgentRepo)
	}
}

func TestDependencySelection_PostgresSuccess(t *testing.T) {
	// Start a real postgres container to satisfy the Ping check
	connStr := testutil.StartPostgresContainer(context.Background(), t)

	cfg := &config.Config{
		DatabaseURL: connStr,
	}

	// This should succeed
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	deps, err := bootstrap.InitializeDependencies(ctx, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer deps.DB.Close()

	if _, ok := deps.AgentRepo.(*postgres.AgentRepository); !ok {
		t.Fatalf("expected AgentRepo to be postgres, got %T", deps.AgentRepo)
	}
}

func TestDependencySelection_PostgresFailure(t *testing.T) {
	cfg := &config.Config{
		DatabaseURL: "postgres://invalid:user@localhost:1234/missing?sslmode=disable",
	}

	// This should fail-fast
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := bootstrap.InitializeDependencies(ctx, cfg)
	if err == nil {
		t.Fatalf("expected error due to unreachable database")
	}
}
