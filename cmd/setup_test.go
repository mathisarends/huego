package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mathisarends/huego/internal/config"
	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

func TestSelectBridgeSingleSkipsPrompt(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(&bytes.Buffer{})

	bridges := []hue.DiscoveredBridge{{ID: "abc", InternalIPAddress: "192.168.1.10"}}
	bridge, err := selectBridge(cmd, bridges)
	if err != nil {
		t.Fatalf("selectBridge() error = %v", err)
	}
	if bridge.InternalIPAddress != "192.168.1.10" {
		t.Fatalf("selectBridge() = %+v, want the only bridge", bridge)
	}
}

func TestSelectBridgeMultiplePromptsForChoice(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader("2\n"))
	buf := &bytes.Buffer{}
	cmd.SetErr(buf)

	bridges := []hue.DiscoveredBridge{
		{ID: "a", InternalIPAddress: "192.168.1.10"},
		{ID: "b", InternalIPAddress: "192.168.1.11"},
	}
	bridge, err := selectBridge(cmd, bridges)
	if err != nil {
		t.Fatalf("selectBridge() error = %v", err)
	}
	if bridge.InternalIPAddress != "192.168.1.11" {
		t.Fatalf("selectBridge() = %+v, want the second bridge", bridge)
	}
	if !strings.Contains(buf.String(), "Found 2 bridges") {
		t.Fatalf("output = %q, want it to list the bridges", buf.String())
	}
}

func TestSelectBridgeRetriesOnInvalidChoice(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader("nope\n1\n"))
	buf := &bytes.Buffer{}
	cmd.SetErr(buf)

	bridges := []hue.DiscoveredBridge{{ID: "a", InternalIPAddress: "192.168.1.10"}, {ID: "b", InternalIPAddress: "192.168.1.11"}}
	bridge, err := selectBridge(cmd, bridges)
	if err != nil {
		t.Fatalf("selectBridge() error = %v", err)
	}
	if bridge.InternalIPAddress != "192.168.1.10" {
		t.Fatalf("selectBridge() = %+v, want the first bridge", bridge)
	}
	if !strings.Contains(buf.String(), "Invalid choice, try again.") {
		t.Fatalf("output = %q, want it to report the invalid choice", buf.String())
	}
}

func TestPrintSetupResultWithClientKey(t *testing.T) {
	buf := &bytes.Buffer{}
	printSetupResult(buf, setupResult{BridgeIP: "192.168.1.10", AppKey: "app-key", ClientKey: "client-key"})
	out := buf.String()
	if !strings.Contains(out, "bridge_ip  = 192.168.1.10") || !strings.Contains(out, "app_key    = app-key") || !strings.Contains(out, "client_key = client-key") {
		t.Fatalf("output = %q, want it to mention IP, app key, and client key", out)
	}
}

func TestPrintSetupResultWithoutClientKey(t *testing.T) {
	buf := &bytes.Buffer{}
	printSetupResult(buf, setupResult{BridgeIP: "192.168.1.10", AppKey: "app-key"})
	if strings.Contains(buf.String(), "client_key") {
		t.Fatalf("output = %q, want no client_key line when there is none", buf.String())
	}
}

func TestSelectBridgeWithoutInputAsksForIP(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader(""))
	cmd.SetErr(&bytes.Buffer{})

	bridges := []hue.DiscoveredBridge{{ID: "a", InternalIPAddress: "192.168.1.10"}, {ID: "b", InternalIPAddress: "192.168.1.11"}}
	_, err := selectBridge(cmd, bridges)
	if err == nil || !strings.Contains(err.Error(), "--ip") {
		t.Fatalf("selectBridge() error = %v, want a hint to pass --ip", err)
	}
}

func TestSetupWithIPSkipsDiscoveryAndSavesKeys(t *testing.T) {
	srv := newFakeBridgeAPI(t)
	var saved config.Bridge
	orig := saveBridge
	t.Cleanup(func() { saveBridge = orig })
	saveBridge = func(b config.Bridge) error { saved = b; return nil }

	out, err := execute(t, "setup", "--ip", srv, "--json")
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if saved.IP != srv || saved.AppKey != "new-app-key" {
		t.Fatalf("saved = %+v, want the given IP and the new app key", saved)
	}
	if !strings.Contains(out, `"app_key":"new-app-key"`) {
		t.Fatalf("output = %q, want the JSON result", out)
	}
}

// newFakeBridgeAPI serves the v1 /api registration endpoint and returns the
// address to pass as --ip.
func newFakeBridgeAPI(t *testing.T) string {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSONFixture(t, w, []map[string]any{{"success": map[string]string{"username": "new-app-key"}}})
	}))
	t.Cleanup(srv.Close)
	return strings.TrimPrefix(srv.URL, "https://")
}
