package router

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// CircuitState represents the operational status of a model circuit.
type CircuitState string

const (
	StateClosed   CircuitState = "CLOSED"    // Normal operation (Healthy)
	StateOpen     CircuitState = "OPEN"      // Tripped due to failures (Unavailable)
	StateHalfOpen CircuitState = "HALF_OPEN" // Probing recovery
)

// InfrastructureError indicates an operational failure that should increment the circuit breaker.
type InfrastructureError struct {
	Code    string
	Message string
}

func (e *InfrastructureError) Error() string {
	return fmt.Sprintf("infra error [%s]: %s", e.Code, e.Message)
}

// ClientError represents request-level invalidity (e.g. 400 bad request, policy denial) that must NOT trip circuit breaker.
type ClientError struct {
	Code    string
	Message string
}

func (e *ClientError) Error() string {
	return fmt.Sprintf("client error [%s]: %s", e.Code, e.Message)
}

// IsInfrastructureError determines if an error is an infrastructure failure vs user/domain error.
func IsInfrastructureError(err error) bool {
	if err == nil {
		return false
	}
	var infraErr *InfrastructureError
	if errors.As(err, &infraErr) {
		return true
	}
	var clientErr *ClientError
	if errors.As(err, &clientErr) {
		return false
	}
	// Common infra error strings
	msg := err.Error()
	return msg == "timeout" || msg == "rate_limit_429" || msg == "server_error_503" || msg == "connection_refused"
}

// CircuitBreaker manages state transitions and health status for a single model endpoint.
type CircuitBreaker struct {
	mu                   sync.RWMutex
	modelID              string
	state                CircuitState
	failureThreshold     int
	cooldownPeriod       time.Duration
	consecutiveFailures  int
	consecutiveSuccesses int
	probeInFlight        bool
	lastStateChangeAt    time.Time
	lastFailureAt        time.Time
	totalInvocations     int64
	totalFailures        int64
}

// NewCircuitBreaker creates a new circuit breaker for a model.
func NewCircuitBreaker(modelID string, failureThreshold int, cooldownPeriod time.Duration) *CircuitBreaker {
	if failureThreshold <= 0 {
		failureThreshold = 3
	}
	if cooldownPeriod <= 0 {
		cooldownPeriod = 5 * time.Second
	}
	return &CircuitBreaker{
		modelID:           modelID,
		state:             StateClosed,
		failureThreshold:  failureThreshold,
		cooldownPeriod:    cooldownPeriod,
		lastStateChangeAt: time.Now().UTC(),
	}
}

// AllowExecution checks whether an invocation is allowed through the circuit breaker.
func (cb *CircuitBreaker) AllowExecution() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now().UTC()
	if cb.state == StateOpen {
		if now.Sub(cb.lastStateChangeAt) >= cb.cooldownPeriod {
			cb.state = StateHalfOpen
			cb.probeInFlight = true
			cb.lastStateChangeAt = now
			return true
		}
		return false
	}

	if cb.state == StateHalfOpen {
		// Single concurrent probe permitted in HalfOpen state
		if cb.probeInFlight {
			return false
		}
		cb.probeInFlight = true
		return true
	}

	return true
}

// RecordSuccess registers a successful model invocation.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalInvocations++
	cb.consecutiveFailures = 0
	cb.probeInFlight = false

	if cb.state == StateHalfOpen {
		cb.consecutiveSuccesses++
		if cb.consecutiveSuccesses >= 2 {
			cb.state = StateClosed
			cb.consecutiveSuccesses = 0
			cb.lastStateChangeAt = time.Now().UTC()
		}
	}
}

// RecordFailure registers an invocation failure.
func (cb *CircuitBreaker) RecordFailure(err error) {
	if !IsInfrastructureError(err) {
		// Domain/validation errors do NOT trip the infrastructure circuit breaker
		return
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now().UTC()
	cb.totalInvocations++
	cb.totalFailures++
	cb.consecutiveFailures++
	cb.consecutiveSuccesses = 0
	cb.lastFailureAt = now
	cb.probeInFlight = false

	if cb.state == StateHalfOpen {
		cb.state = StateOpen
		cb.lastStateChangeAt = now
	} else if cb.state == StateClosed && cb.consecutiveFailures >= cb.failureThreshold {
		cb.state = StateOpen
		cb.lastStateChangeAt = now
	}
}

// State returns the current internal circuit state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// HealthStatus maps the internal circuit state to model registry health status.
func (cb *CircuitBreaker) HealthStatus() ModelHealthStatus {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateOpen:
		return HealthUnavailable
	case StateHalfOpen:
		return HealthDegraded
	default:
		return HealthHealthy
	}
}

// Metrics returns the observed operational metrics.
func (cb *CircuitBreaker) Metrics() ObservedModelMetrics {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	errorRate := 0.0
	if cb.totalInvocations > 0 {
		errorRate = float64(cb.totalFailures) / float64(cb.totalInvocations)
	}

	return ObservedModelMetrics{
		ObservedErrorRate:   errorRate,
		ConsecutiveFailures: cb.consecutiveFailures,
		LastFailureAt:       cb.lastFailureAt,
		TotalInvocations:    cb.totalInvocations,
	}
}

// BreakerRegistry manages circuit breakers for all registered models.
type BreakerRegistry struct {
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
}

// NewBreakerRegistry initializes a BreakerRegistry.
func NewBreakerRegistry() *BreakerRegistry {
	return &BreakerRegistry{
		breakers: make(map[string]*CircuitBreaker),
	}
}

// GetOrCreate returns the circuit breaker for a given model ID.
func (br *BreakerRegistry) GetOrCreate(modelID string) *CircuitBreaker {
	br.mu.Lock()
	defer br.mu.Unlock()

	cb, ok := br.breakers[modelID]
	if !ok {
		cb = NewCircuitBreaker(modelID, 3, 5*time.Second)
		br.breakers[modelID] = cb
	}
	return cb
}
