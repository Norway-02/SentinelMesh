package memory_test

import (
	"testing"

	"github.com/sentinelmesh/sentinelmesh/internal/repository"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/contract"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
)

func TestAgentRepositoryContract(t *testing.T) {
	contract.RunAgentRepositoryContractTests(t, func(t *testing.T) repository.AgentRepository {
		return memory.NewAgentRepository()
	})
}
