package domain

import (
	"fmt"
	"strings"
)

// ValidateIdentifier ensures an ID is not empty and contains no spaces.
func ValidateIdentifier(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("identifier cannot be empty")
	}
	if strings.ContainsAny(id, " \t\n\r") {
		return fmt.Errorf("identifier cannot contain whitespace")
	}
	return nil
}
