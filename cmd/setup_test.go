package cmd

import (
	"bytes"
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
	cmd.SetOut(buf)

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
	cmd.SetOut(buf)

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
	printSetupResult(buf, config.Bridge{IP: "192.168.1.10", AppKey: "app-key", ClientKey: "client-key"})
	out := buf.String()
	if !strings.Contains(out, "bridge_ip = 192.168.1.10") || !strings.Contains(out, "app_key   = app-key") || !strings.Contains(out, "client_key = client-key") {
		t.Fatalf("output = %q, want it to mention IP, app key, and client key", out)
	}
}

func TestPrintSetupResultWithoutClientKey(t *testing.T) {
	buf := &bytes.Buffer{}
	printSetupResult(buf, config.Bridge{IP: "192.168.1.10", AppKey: "app-key"})
	if strings.Contains(buf.String(), "client_key") {
		t.Fatalf("output = %q, want no client_key line when there is none", buf.String())
	}
}
