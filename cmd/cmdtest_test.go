package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mathisarends/huego/internal/config"
	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func writeJSONFixture(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

// newFakeClient starts a fake Hue bridge server and points a real *hue.Client
// at it.
func newFakeClient(t *testing.T, handler http.HandlerFunc) *hue.Client {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)

	ip := strings.TrimPrefix(srv.URL, "https://")
	return hue.NewClient(config.Bridge{IP: ip, AppKey: "test-app-key"})
}

// withClient overrides newClient for the duration of the test.
func withClient(t *testing.T, c *hue.Client) {
	t.Helper()
	orig := newClient
	t.Cleanup(func() { newClient = orig })
	newClient = func() (*hue.Client, error) { return c, nil }
}

// execute runs the root command with the given args and returns everything
// written to its combined stdout/stderr. Flags are reset to their defaults
// first since pflag values otherwise persist across Execute() calls.
func execute(t *testing.T, args ...string) (string, error) {
	t.Helper()
	prepareCommands()
	resetFlags(rootCmd)

	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return buf.String(), err
}

func resetFlags(c *cobra.Command) {
	c.Flags().VisitAll(func(f *pflag.Flag) {
		// Setting a slice flag to its "[]" default would add a literal "[]".
		if slice, ok := f.Value.(pflag.SliceValue); ok {
			_ = slice.Replace(nil)
		} else {
			_ = f.Value.Set(f.DefValue)
		}
		f.Changed = false
	})
	for _, sub := range c.Commands() {
		resetFlags(sub)
	}
}
