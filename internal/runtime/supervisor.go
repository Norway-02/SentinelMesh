package runtime

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

// StateChangeCallback is invoked whenever an agent run transitions lifecycle state.
type StateChangeCallback func(status ExecutionStatus)

type monitoredRun struct {
	request       ExecutionRequest
	lastHeartbeat time.Time
	deadline      time.Time
	lastState     types.AgentState
	stopping      bool
}

// Supervisor monitors active agent executions, enforces timeouts, tracks heartbeats,
// and broadcasts state transitions to downstream state synchronizers.
type Supervisor struct {
	mu           sync.RWMutex
	runtime      Runtime
	runs         map[string]*monitoredRun
	pollInterval time.Duration
	callbacks    []StateChangeCallback
	stopCh       chan struct{}
	running      bool
}

// NewSupervisor constructs a new execution Supervisor.
func NewSupervisor(r Runtime, pollInterval time.Duration) *Supervisor {
	if pollInterval <= 0 {
		pollInterval = 500 * time.Millisecond
	}
	return &Supervisor{
		runtime:      r,
		runs:         make(map[string]*monitoredRun),
		pollInterval: pollInterval,
		stopCh:       make(chan struct{}),
	}
}

// OnStateChange registers a callback triggered on state transitions.
func (s *Supervisor) OnStateChange(cb StateChangeCallback) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callbacks = append(s.callbacks, cb)
}

// Start begins the supervision watchdog loop.
func (s *Supervisor) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	go s.supervisionLoop(ctx)
}

// Stop terminates the supervisor loop.
func (s *Supervisor) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
	close(s.stopCh)
}

// Register tracks a newly started execution.
func (s *Supervisor) Register(req ExecutionRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var deadline time.Time
	if req.Timeout > 0 {
		deadline = now.Add(req.Timeout)
	}

	s.runs[req.RunID] = &monitoredRun{
		request:       req,
		lastHeartbeat: now,
		deadline:      deadline,
		lastState:     types.StateStarting,
	}
}

// Heartbeat refreshes the liveness timestamp of a running agent workload.
func (s *Supervisor) Heartbeat(runID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if item, exists := s.runs[runID]; exists {
		item.lastHeartbeat = time.Now()
	}
}

func (s *Supervisor) supervisionLoop(ctx context.Context) {
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkRuns(ctx)
		}
	}
}

func (s *Supervisor) checkRuns(ctx context.Context) {
	s.mu.RLock()
	activeRuns := make(map[string]monitoredRun, len(s.runs))
	for k, v := range s.runs {
		activeRuns[k] = *v
	}
	s.mu.RUnlock()

	now := time.Now()

	for runID, mon := range activeRuns {
		// 1. Check timeout deadline
		if !mon.deadline.IsZero() && now.After(mon.deadline) && mon.lastState.IsNonTerminal() && !mon.stopping {
			s.mu.Lock()
			if item, exists := s.runs[runID]; exists {
				item.stopping = true
			}
			s.mu.Unlock()

			slog.Warn("Run exceeded timeout deadline, stopping", "run_id", runID, "deadline", mon.deadline)
			go func(id string) {
				_ = s.runtime.Stop(ctx, id, 500*time.Millisecond)
			}(runID)
		}

		// 2. Poll runtime status
		status, err := s.runtime.Status(ctx, runID)
		if err != nil {
			continue
		}

		// 3. Detect state transition
		if status.State != mon.lastState {
			s.mu.Lock()
			if item, exists := s.runs[runID]; exists {
				item.lastState = status.State
			}
			s.mu.Unlock()

			s.notifyCallbacks(*status)

			// Clean up finished runs from active supervisor monitoring
			if status.IsFinished() {
				s.mu.Lock()
				delete(s.runs, runID)
				s.mu.Unlock()
			}
		}
	}
}

func (s *Supervisor) notifyCallbacks(status ExecutionStatus) {
	s.mu.RLock()
	callbacks := make([]StateChangeCallback, len(s.callbacks))
	copy(callbacks, s.callbacks)
	s.mu.RUnlock()

	for _, cb := range callbacks {
		cb(status)
	}
}
