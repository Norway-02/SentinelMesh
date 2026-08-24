package memory_test

import (
	"testing"

	"github.com/sentinelmesh/sentinelmesh/internal/repository"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/contract"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
)

func TestRunRepositoryContract(t *testing.T) {
	contract.RunRunRepositoryContractTests(t, func(t *testing.T) repository.RunRepository {
		return memory.NewRunRepository()
	})
}
