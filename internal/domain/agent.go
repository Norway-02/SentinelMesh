package domain

import (
	"fmt"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

// Agent aggregate represents an AI agent workload definition.
type Agent struct {
	ID                 string                   `json:"id"`
	Name               string                   `json:"name"`
	Version            string                   `json:"version"`
	TenantID           string                   `json:"tenant_id"`
	Image              string                   `json:"image"`
	Resources          types.AgentResources     `json:"resources"`
	Priority           string                   `json:"priority"`
	SecurityPolicy     types.SecurityPolicy     `json:"security_policy"`
	NetworkPolicy      types.NetworkPolicy      `json:"network_policy"`
	CheckpointPolicy   types.CheckpointPolicy   `json:"checkpoint_policy"`
	VerificationPolicy types.VerificationPolicy `json:"verification_policy"`
	ModelPolicy        string                   `json:"model_policy,omitempty"`
	State              string                   `json:"state"`
	CreatedAt          time.Time                `json:"created_at"`
	UpdatedAt          time.Time                `json:"updated_at"`
}

// Validate checks the deterministic correctness of an Agent aggregate.
func (a *Agent) Validate() error {
	if err := ValidateIdentifier(a.ID); err != nil {
		return fmt.Errorf("%w: invalid agent ID: %v", ErrInvalidAgent, err)
	}
	if a.Name == "" {
		return fmt.Errorf("%w: name cannot be empty", ErrInvalidAgent)
	}
	if a.Version == "" {
		return fmt.Errorf("%w: version cannot be empty", ErrInvalidAgent)
	}

	// Validate Resources
	if a.Resources.CPU == "" || a.Resources.CPU[:1] == "-" {
		// Basic check for negative CPU or empty. In a real system, we'd parse "500m" or "2".
		return fmt.Errorf("%w: invalid CPU resource", ErrInvalidAgent)
	}
	if a.Resources.Memory == "" || a.Resources.Memory[:1] == "-" {
		return fmt.Errorf("%w: invalid Memory resource", ErrInvalidAgent)
	}
	if a.Resources.GPU < 0 {
		return fmt.Errorf("%w: negative GPU values are not allowed", ErrInvalidAgent)
	}

	// Validate Priority
	if a.Priority != "low" && a.Priority != "normal" && a.Priority != "high" && a.Priority != "critical" {
		return fmt.Errorf("%w: invalid priority '%s'", ErrInvalidAgent, a.Priority)
	}

	// Validate CheckpointPolicy
	if a.CheckpointPolicy.Enabled {
		if a.CheckpointPolicy.Interval == "" || a.CheckpointPolicy.Interval[:1] == "-" {
			return fmt.Errorf("%w: invalid checkpoint interval", ErrInvalidAgent)
		}
	}

	return nil
}
