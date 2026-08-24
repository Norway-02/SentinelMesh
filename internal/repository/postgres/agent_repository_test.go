package postgres_test

import (
	"testing"

	"github.com/sentinelmesh/sentinelmesh/internal/repository"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/contract"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/postgres"
	"github.com/sentinelmesh/sentinelmesh/internal/testutil"
)

func TestAgentRepositoryContract(t *testing.T) {
	db := testutil.SetupTestDB(t)

	contract.RunAgentRepositoryContractTests(t, func(t *testing.T) repository.AgentRepository {
		// Clean up the table before each test in the suite if needed, 
		// but TestContainers DB is isolated per SetupTestDB, which runs per test package,
		// wait, SetupTestDB runs once per TestAgentRepositoryContract.
		// Since contract tests might conflict, we can delete all agents.
		_, err := db.Exec("DELETE FROM agent_runs; DELETE FROM agents;")
		if err != nil {
			t.Fatalf("failed to clean db: %v", err)
		}
		
		return postgres.NewAgentRepository(db)
	})
}
