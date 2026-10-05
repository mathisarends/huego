package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"

	"github.com/mathisarends/huego/internal/config"
	"github.com/mathisarends/huego/internal/hue"
)

func commandProblem(err error) *hue.Problem {
	var problem *hue.Problem
	if errors.As(err, &problem) {
		return problem
	}
	result := &hue.Problem{Code: "invalid_argument", Message: err.Error()}
	var api *hue.APIError
	var missing *hue.ResourceNotFoundError
	var network net.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		result.Code = "timeout"
	case errors.Is(err, context.Canceled):
		result.Code = "canceled"
	case errors.Is(err, config.ErrNotConfigured):
		result.Code = "not_configured"
	case errors.As(err, &api):
		result.Code = "bridge_http_error"
		result.Details = map[string]any{"http_status": api.StatusCode, "body": api.Message}
	case errors.As(err, &missing):
		result.Code = "target_not_found"
	case errors.As(err, &network):
		result.Code = "bridge_unreachable"
	case errors.Is(err, os.ErrNotExist), errors.Is(err, os.ErrPermission):
		result.Code = "file_error"
	}
	return result
}

func writeCommandError(w io.Writer, err error) {
	_ = json.NewEncoder(w).Encode(map[string]any{"error": commandProblem(err)})
}
