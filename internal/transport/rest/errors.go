package rest

import (
	"errors"
	"net/http"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
)

type APIError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"-"`
}

func MapError(err error) APIError {
	if err == nil {
		return APIError{Code: http.StatusOK}
	}

	// Repository errors
	if errors.Is(err, repository.ErrNotFound) {
		return APIError{Error: "NotFound", Message: "resource not found", Code: http.StatusNotFound}
	}
	if errors.Is(err, repository.ErrAlreadyExists) {
		return APIError{Error: "AlreadyExists", Message: "resource already exists", Code: http.StatusConflict}
	}

	// Domain errors
	if errors.Is(err, domain.ErrInvalidAgent) || errors.Is(err, domain.ErrInvalidAgentRun) {
		return APIError{Error: "InvalidArgument", Message: err.Error(), Code: http.StatusBadRequest}
	}
	if errors.Is(err, domain.ErrInvalidStateTransition) {
		return APIError{Error: "FailedPrecondition", Message: err.Error(), Code: http.StatusPreconditionFailed}
	}
	if errors.Is(err, domain.ErrRetryLimitExceeded) {
		return APIError{Error: "ResourceExhausted", Message: err.Error(), Code: http.StatusTooManyRequests}
	}

	// Fallback for internal errors
	return APIError{Error: "InternalError", Message: "internal server error", Code: http.StatusInternalServerError}
}

func WriteError(w http.ResponseWriter, err error) {
	apiErr := MapError(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(apiErr.Code)
	// Write JSON error
	// A robust implementation would use encoding/json here
	w.Write([]byte(`{"error":"` + apiErr.Error + `","message":"` + apiErr.Message + `"}`))
}
