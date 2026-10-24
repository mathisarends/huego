package hue

import (
	"fmt"
	"strings"
)

type ResourceNotFoundError struct {
	Message string
}

func (e *ResourceNotFoundError) Error() string { return e.Message }

func newResourceNotFoundError(format string, args ...any) error {
	return &ResourceNotFoundError{Message: fmt.Sprintf(format, args...)}
}

// APIError is returned for any non-2xx response from the bridge's CLIP v2 API.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("hue bridge error (%d): %s", e.StatusCode, e.Message)
}

// AmbiguousNameError is returned when a name matches more than one resource.
// Candidates holds identifiers the caller can pass instead to disambiguate.
type AmbiguousNameError struct {
	ResourceType string
	Name         string
	Candidates   []string
}

func (e *AmbiguousNameError) Error() string {
	return fmt.Sprintf("%d %ss are named %q; use one of: %s", len(e.Candidates), e.ResourceType, e.Name, strings.Join(e.Candidates, ", "))
}
