package repository

import (
	"context"
	"errors"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrConflict      = errors.New("conflict")
)

type AgentFilter struct {
	TenantID  string
	PageToken string
	PageSize  int
}

type AgentRepository interface {
	Create(ctx context.Context, agent domain.Agent) error
	Get(ctx context.Context, id string) (domain.Agent, error)
	List(ctx context.Context, filter AgentFilter) ([]domain.Agent, string, error)
	Delete(ctx context.Context, id string) error
}
