package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/mathisarends/huego/internal/config"
	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

// Exit codes let scripts and agents branch on the kind of failure without
// parsing messages.
const (
	exitError       = 1
	exitUsage       = 2
	exitNotFound    = 3
	exitAuth        = 4
	exitUnreachable = 5
)

// usageError marks invalid input: wrong arguments, flags, or values. It is
// raised before any bridge request is made.
type usageError struct{ err error }

func (e usageError) Error() string { return e.err.Error() }
func (e usageError) Unwrap() error { return e.err }

func asUsage(err error) error {
	if err == nil {
		return nil
	}
	var usage usageError
	if errors.As(err, &usage) {
		return err
	}
	return usageError{err}
}

func usagef(format string, args ...any) error {
	return usageError{fmt.Errorf(format, args...)}
}

// partialUpdateError is returned when the bridge accepted a request but
// reported errors for part of it.
type partialUpdateError struct{ message string }

func (e partialUpdateError) Error() string { return e.message }

// errorReport is the --json error object written to stderr.
type errorReport struct {
	Error      string   `json:"error"`
	Code       string   `json:"code"`
	Candidates []string `json:"candidates,omitempty"`
}

func classifyError(err error) (errorReport, int) {
	report := errorReport{Error: err.Error()}
	var (
		usage     usageError
		notFound  *hue.ResourceNotFoundError
		ambiguous *hue.AmbiguousNameError
		apiErr    *hue.APIError
		partial   partialUpdateError
		netErr    net.Error
		opErr     *net.OpError
	)
	switch {
	case errors.As(err, &usage):
		report.Code = "usage"
		return report, exitUsage
	case errors.As(err, &ambiguous):
		report.Code = "ambiguous"
		report.Candidates = ambiguous.Candidates
		return report, exitNotFound
	case errors.As(err, &notFound), errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound:
		report.Code = "not_found"
		return report, exitNotFound
	case errors.Is(err, config.ErrNotConfigured):
		report.Code = "not_configured"
		return report, exitAuth
	case hue.IsAuthError(err):
		report.Code = "unauthorized"
		return report, exitAuth
	case errors.As(err, &apiErr), errors.As(err, &partial):
		report.Code = "bridge_error"
		return report, exitError
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &opErr), errors.As(err, &netErr) && netErr.Timeout():
		report.Code = "unreachable"
		return report, exitUnreachable
	}
	report.Code = "error"
	return report, exitError
}

// reportError writes err to w, as JSON when asJSON is set, and returns the
// process exit code for it.
func reportError(w io.Writer, err error, asJSON bool) int {
	report, code := classifyError(err)
	if asJSON {
		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(false)
		_ = encoder.Encode(report)
		return code
	}
	_, _ = fmt.Fprintln(w, "error:", err)
	return code
}

var prepareOnce sync.Once

// prepareCommands finishes the command tree once every init has registered
// its commands; it cannot run from an init itself because init order across
// files would leave later commands out.
func prepareCommands() {
	prepareOnce.Do(func() {
		rootCmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error { return asUsage(err) })
		markUsageErrors(rootCmd)
		// The RunE markUsageErrors gives group commands makes cobra list a
		// bare "huego light [flags]" usage line, which only prints help.
		rootCmd.SetUsageTemplate(strings.Replace(rootCmd.UsageTemplate(),
			"{{if .Runnable}}", "{{if and .Runnable (not .HasAvailableSubCommands)}}", 1))
		installCompletions(rootCmd, nil)
	})
}

// markUsageErrors makes cobra's argument-count errors classifiable as usage
// errors, and makes group commands such as "light" reject unknown
// subcommands instead of silently printing help.
func markUsageErrors(cmd *cobra.Command) {
	if validate := cmd.Args; validate != nil {
		cmd.Args = func(c *cobra.Command, args []string) error {
			if err := validate(c, args); err != nil {
				return usagef("%w\nusage: %s", err, c.UseLine())
			}
			return nil
		}
	}
	if cmd.HasSubCommands() && !cmd.Runnable() {
		cmd.Args = cobra.ArbitraryArgs
		cmd.RunE = func(c *cobra.Command, args []string) error {
			if len(args) == 0 {
				return c.Help()
			}
			message := fmt.Sprintf("unknown command %q for %q", args[0], c.CommandPath())
			if suggestions := c.SuggestionsFor(args[0]); len(suggestions) > 0 {
				message += "; did you mean " + strings.Join(suggestions, " or ") + "?"
			}
			return usagef("%s", message)
		}
	}
	for _, sub := range cmd.Commands() {
		markUsageErrors(sub)
	}
}
