package hue

import (
	"net/http"
	"strings"
	"testing"
)

func TestResourceNotFoundErrorMessage(t *testing.T) {
	err := newResourceNotFoundError("no %s with ID %s", "light", "abc")
	if err.Error() != "no light with ID abc" {
		t.Fatalf("Error() = %q, want %q", err.Error(), "no light with ID abc")
	}
}

func TestAPIErrorMessage(t *testing.T) {
	err := &APIError{StatusCode: http.StatusUnauthorized, Message: "unauthorized user"}
	got := err.Error()
	if !strings.Contains(got, "401") || !strings.Contains(got, "unauthorized user") {
		t.Fatalf("Error() = %q, want it to mention the status code and message", got)
	}
}
