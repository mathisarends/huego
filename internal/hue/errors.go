package hue

import "fmt"

type Problem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func (e *Problem) Error() string { return e.Message }

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
