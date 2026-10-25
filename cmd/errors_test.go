package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/mathisarends/huego/internal/config"
	"github.com/mathisarends/huego/internal/hue"
)

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode string
		wantExit int
	}{
		{"usage", usagef("bad input"), "usage", exitUsage},
		{"not found", &hue.ResourceNotFoundError{Message: "no light named x"}, "not_found", exitNotFound},
		{"ambiguous", &hue.AmbiguousNameError{ResourceType: "light", Name: "x", Candidates: []string{"a", "b"}}, "ambiguous", exitNotFound},
		{"not configured", config.ErrNotConfigured, "not_configured", exitAuth},
		{"unauthorized", &hue.APIError{StatusCode: http.StatusForbidden}, "unauthorized", exitAuth},
		{"missing resource", &hue.APIError{StatusCode: http.StatusNotFound}, "not_found", exitNotFound},
		{"bridge", &hue.APIError{StatusCode: http.StatusInternalServerError}, "bridge_error", exitError},
		{"partial", partialUpdateError{"some lights failed"}, "bridge_error", exitError},
		{"unreachable", fmt.Errorf("get: %w", &net.OpError{Op: "dial", Err: errors.New("connection refused")}), "unreachable", exitUnreachable},
		{"other", errors.New("boom"), "error", exitError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, exit := classifyError(tt.err)
			if report.Code != tt.wantCode || exit != tt.wantExit {
				t.Fatalf("classifyError() = %q, %d; want %q, %d", report.Code, exit, tt.wantCode, tt.wantExit)
			}
		})
	}
}

func TestReportErrorJSONIncludesCandidates(t *testing.T) {
	var buf bytes.Buffer
	exit := reportError(&buf, &hue.AmbiguousNameError{ResourceType: "scene", Name: "Relax", Candidates: []string{"Kitchen/Relax", "Office/Relax"}}, true)

	var report errorReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("output %q is not JSON: %v", buf.String(), err)
	}
	if exit != exitNotFound || report.Code != "ambiguous" || len(report.Candidates) != 2 {
		t.Fatalf("report = %+v, exit = %d; want ambiguous with 2 candidates", report, exit)
	}
}

func TestUsageErrorsFromCobra(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"light", "on"}, "usage: huego light on <target>"},
		{[]string{"light", "of", "Desk"}, `did you mean off?`},
		{[]string{"lamp"}, `unknown command "lamp"`},
		{[]string{"light", "on", "Desk", "--bogus"}, "unknown flag"},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			_, err := execute(t, tt.args...)
			var usage usageError
			if !errors.As(err, &usage) {
				t.Fatalf("error = %v (%T), want a usage error", err, err)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestGroupHelpListsOnlySubcommandUsage(t *testing.T) {
	out, err := execute(t, "light", "--help")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "huego light [flags]") || !strings.Contains(out, "huego light [command]") {
		t.Errorf("light --help usage:\n%s", out)
	}

	out, err = execute(t, "light", "on", "--help")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "huego light on <target> [flags]") {
		t.Errorf("light on --help usage:\n%s", out)
	}
}
