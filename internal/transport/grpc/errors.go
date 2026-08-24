package grpc

import (
	"errors"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MapError translates domain and repository errors into gRPC status errors.
func MapError(err error) error {
	if err == nil {
		return nil
	}

	// Repository errors
	if errors.Is(err, repository.ErrNotFound) {
		return status.Error(codes.NotFound, "resource not found")
	}
	if errors.Is(err, repository.ErrAlreadyExists) {
		return status.Error(codes.AlreadyExists, "resource already exists")
	}

	// Domain errors
	if errors.Is(err, domain.ErrInvalidAgent) || errors.Is(err, domain.ErrInvalidAgentRun) {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	if errors.Is(err, domain.ErrInvalidStateTransition) {
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	if errors.Is(err, domain.ErrRetryLimitExceeded) {
		return status.Error(codes.ResourceExhausted, err.Error())
	}

	// Fallback for internal errors
	return status.Error(codes.Internal, "internal server error")
}
