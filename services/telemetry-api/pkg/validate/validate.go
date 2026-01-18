// Package validate provides input validation utilities.
package validate

import (
	"fmt"
	"regexp"
	"strings"
)

// Common validation patterns
var (
	// Identifier allows alphanumeric, underscores, and hyphens (safe for IDs)
	identifierPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)

	// Version allows semver-like patterns (1.0.0, v1.2.3, 2024.01.15)
	versionPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,31}$`)

	// UUID pattern
	uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

// Identifier validates that a string is a safe identifier.
// Rules:
// - Starts with a letter
// - Contains only letters, numbers, underscores, hyphens
// - 1-64 characters long
func Identifier(s string) error {
	if s == "" {
		return fmt.Errorf("identifier cannot be empty")
	}
	if !identifierPattern.MatchString(s) {
		return fmt.Errorf("invalid identifier %q: must start with a letter and contain only letters, numbers, underscores, and hyphens (max 64 chars)", s)
	}
	return nil
}

// Version validates that a string is a safe version identifier.
// Rules:
// - Starts with a letter or number
// - Contains only letters, numbers, dots, underscores, hyphens
// - 1-32 characters long
func Version(s string) error {
	if s == "" {
		return fmt.Errorf("version cannot be empty")
	}
	if !versionPattern.MatchString(s) {
		return fmt.Errorf("invalid version %q: must start with a letter or number and contain only letters, numbers, dots, underscores, and hyphens (max 32 chars)", s)
	}
	return nil
}

// UUID validates that a string is a valid UUID.
func UUID(s string) error {
	if s == "" {
		return fmt.Errorf("UUID cannot be empty")
	}
	if !uuidPattern.MatchString(s) {
		return fmt.Errorf("invalid UUID format: %q", s)
	}
	return nil
}

// PathSegment validates that a string is safe to use as a path segment.
// This is more restrictive than Identifier to prevent path traversal.
// Rules:
// - Not empty
// - Does not contain . or /
// - Does not start with -
// - 1-64 characters
func PathSegment(s string) error {
	if s == "" {
		return fmt.Errorf("path segment cannot be empty")
	}
	if len(s) > 64 {
		return fmt.Errorf("path segment too long (max 64 chars)")
	}
	if strings.Contains(s, "/") {
		return fmt.Errorf("path segment cannot contain /")
	}
	if strings.Contains(s, "..") {
		return fmt.Errorf("path segment cannot contain ..")
	}
	if strings.HasPrefix(s, ".") {
		return fmt.Errorf("path segment cannot start with .")
	}
	if strings.HasPrefix(s, "-") {
		return fmt.Errorf("path segment cannot start with -")
	}
	return nil
}

// CommandType validates that a command type is safe.
// Uses Identifier rules plus additional checks.
func CommandType(s string) error {
	if err := Identifier(s); err != nil {
		return fmt.Errorf("invalid command type: %w", err)
	}
	// Additional restrictions for command types
	lower := strings.ToLower(s)
	dangerousTypes := []string{"exec", "shell", "eval", "system"}
	for _, d := range dangerousTypes {
		if strings.Contains(lower, d) {
			return fmt.Errorf("command type cannot contain %q", d)
		}
	}
	return nil
}

// TelemetryType validates that a telemetry type is safe.
func TelemetryType(s string) error {
	if s == "" {
		return fmt.Errorf("telemetry type cannot be empty")
	}
	// Allow common telemetry types with underscores
	if !identifierPattern.MatchString(s) {
		return fmt.Errorf("invalid telemetry type %q: must start with a letter and contain only letters, numbers, underscores, and hyphens", s)
	}
	return nil
}

// StringLength validates string length is within bounds.
func StringLength(s string, min, max int) error {
	if len(s) < min {
		return fmt.Errorf("string too short (min %d chars)", min)
	}
	if len(s) > max {
		return fmt.Errorf("string too long (max %d chars)", max)
	}
	return nil
}

// ValidationError represents a validation error with field context.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// NewValidationError creates a new ValidationError.
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{Field: field, Message: message}
}
