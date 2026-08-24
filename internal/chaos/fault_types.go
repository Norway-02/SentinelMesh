package chaos

import (
	"math/rand"
	"sync"
	"time"
)

// FaultType enumerates the classes of injectable faults.
type FaultType string

const (
	FaultDrop      FaultType = "drop"      // Discard/drop operation or event
	FaultDelay     FaultType = "delay"     // Inject latency before executing operation
	FaultError     FaultType = "error"     // Return configured error directly
	FaultCorrupt   FaultType = "corrupt"   // Alter checksum or bit-flip payload
	FaultTruncate  FaultType = "truncate"  // Truncate or malform state payload
	FaultDuplicate FaultType = "duplicate" // Deliver operation or event twice
	FaultPanic     FaultType = "panic"     // Trigger recoverable panic
)

// FaultSpec defines the exact trigger conditions and parameters for a fault.
type FaultSpec struct {
	Type        FaultType     `json:"type"`
	TargetOp    string        `json:"target_op"`   // e.g. "Update", "Insert", "GetLatest"
	Probability float64       `json:"probability"` // 0.0 to 1.0 (1.0 = deterministic always)
	Delay       time.Duration `json:"delay"`       // Injected latency
	ErrorMsg    string        `json:"error_msg"`   // Error returned when FaultError
	After       int           `json:"after"`       // Trigger only after N successful invocations
	Limit       int           `json:"limit"`       // Max number of times this fault will trigger (0 = unlimited)
}

// ExperimentConfig defines the deterministic input parameters for a chaos scenario.
type ExperimentConfig struct {
	ScenarioID  string      `json:"scenario_id"`
	Description string      `json:"description"`
	Seed        int64       `json:"seed"`
	Faults      []FaultSpec `json:"faults"`
}

// FaultController manages thread-safe deterministic fault evaluation using a seeded PRNG.
type FaultController struct {
	mu           sync.Mutex
	seed         int64
	rng          *rand.Rand
	faults       []FaultSpec
	callCounts   map[string]int // targetOp -> call count
	triggerCount map[string]int // targetOp -> trigger count
	enabled      bool
}

// NewFaultController creates a deterministic FaultController with the given seed and faults.
func NewFaultController(seed int64, faults []FaultSpec) *FaultController {
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	return &FaultController{
		seed:         seed,
		rng:          rand.New(rand.NewSource(seed)),
		faults:       faults,
		callCounts:   make(map[string]int),
		triggerCount: make(map[string]int),
		enabled:      true,
	}
}

// SetEnabled dynamically enables or disables fault injection.
func (c *FaultController) SetEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = enabled
}

// ShouldTrigger determines deterministically whether a fault should fire for a target operation.
func (c *FaultController) ShouldTrigger(targetOp string) (bool, *FaultSpec) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.enabled {
		return false, nil
	}

	c.callCounts[targetOp]++
	currentCall := c.callCounts[targetOp]

	for i := range c.faults {
		f := &c.faults[i]
		if f.TargetOp != "" && f.TargetOp != targetOp {
			continue
		}

		if f.After > 0 && currentCall <= f.After {
			continue
		}

		if f.Limit > 0 && c.triggerCount[targetOp] >= f.Limit {
			continue
		}

		// Evaluate probability
		if f.Probability >= 1.0 || c.rng.Float64() < f.Probability {
			c.triggerCount[targetOp]++
			return true, f
		}
	}

	return false, nil
}

// Reset clears call counts and resets the PRNG source.
func (c *FaultController) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rng = rand.New(rand.NewSource(c.seed))
	c.callCounts = make(map[string]int)
	c.triggerCount = make(map[string]int)
}
