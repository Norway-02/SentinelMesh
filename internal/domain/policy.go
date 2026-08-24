package domain

import (
	"fmt"
)

// Policy defines boundaries for an agent's execution.
type Policy struct {
	ID              string
	TenantID        string
	FilesystemRules []string
	NetworkRules    []string
	ToolRules       []string
	ResourceRules   []string
	SecretRules     []string
	ApprovalRules   []string
}

// Validate checks deterministic rules for a Policy object.
func (p *Policy) Validate() error {
	if err := ValidateIdentifier(p.ID); err != nil {
		return fmt.Errorf("%w: invalid policy ID: %v", ErrInvalidPolicy, err)
	}
	if err := ValidateIdentifier(p.TenantID); err != nil {
		return fmt.Errorf("%w: invalid tenant ID: %v", ErrInvalidPolicy, err)
	}

	// Basic validation logic for rules. A more comprehensive system would 
	// parse these rules into typed structs.
	for _, rule := range p.FilesystemRules {
		if rule == "" {
			return fmt.Errorf("%w: empty filesystem rule", ErrInvalidPolicy)
		}
	}
	for _, rule := range p.NetworkRules {
		if rule == "" {
			return fmt.Errorf("%w: empty network rule", ErrInvalidPolicy)
		}
	}

	return nil
}
