package cmd

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mathisarends/huego/internal/omarchy"
)

func omarchyFixture(t *testing.T) string {
	t.Helper()
	home := withOmarchyHome(t)
	files := map[string]string{
		"hypr/hyprland.conf":  "source = ~/.config/hypr/bindings.conf\n",
		"waybar/config.jsonc": "{\n  \"modules-right\": [\n    \"clock\"\n  ]\n}\n",
		"waybar/style.css":    "* {}\n",
	}
	for path, content := range files {
		full := filepath.Join(home, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func withOmarchyHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	orig := omarchyConfigHome
	t.Cleanup(func() { omarchyConfigHome = orig })
	omarchyConfigHome = func() (string, error) { return home, nil }
	return home
}

func countWaybarRestarts(t *testing.T, available bool) *int {
	t.Helper()
	restarts := 0
	orig := restartWaybar
	t.Cleanup(func() { restartWaybar = orig })
	restartWaybar = func(context.Context) (bool, error) {
		if available {
			restarts++
		}
		return available, nil
	}
	return &restarts
}

func runOmarchyJSON(t *testing.T, args ...string) omarchyResult {
	t.Helper()
	out, err := execute(t, append(args, "--json")...)
	if err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out)
	}
	var result omarchyResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output %q is not JSON: %v", out, err)
	}
	return result
}

func TestOmarchyInstallAndUninstall(t *testing.T) {
	home := omarchyFixture(t)
	restarts := countWaybarRestarts(t, true)

	result := runOmarchyJSON(t, "omarchy", "install", "--modifier", "SUPER SHIFT")
	if !result.WaybarRestarted || *restarts != 1 {
		t.Errorf("waybar_restarted = %v after %d restarts, want one restart", result.WaybarRestarted, *restarts)
	}
	bindings, err := os.ReadFile(filepath.Join(home, "hypr", "huego.conf"))
	if err != nil || !strings.Contains(string(bindings), "bindd = SUPER SHIFT, H,") {
		t.Errorf("huego.conf = %q, %v", bindings, err)
	}

	again := runOmarchyJSON(t, "omarchy", "install", "--modifier", "SUPER SHIFT")
	for _, c := range again.Changes {
		if c.Action != omarchy.Unchanged {
			t.Errorf("second install changed %+v", c)
		}
	}
	if *restarts != 1 {
		t.Errorf("Waybar restarted although nothing changed")
	}

	removed := runOmarchyJSON(t, "omarchy", "uninstall")
	if len(removed.Changes) != 4 || !removed.WaybarRestarted {
		t.Errorf("uninstall = %+v", removed)
	}
	config, _ := os.ReadFile(filepath.Join(home, "waybar", "config.jsonc"))
	if strings.Contains(string(config), "custom/hue") {
		t.Errorf("config.jsonc still has the module:\n%s", config)
	}
}

func TestOmarchyInstallDryRunAndRestartHint(t *testing.T) {
	home := omarchyFixture(t)
	restarts := countWaybarRestarts(t, false)

	out, err := execute(t, "omarchy", "install", "--dry-run")
	if err != nil {
		t.Fatalf("install --dry-run: %v", err)
	}
	if !strings.Contains(out, "Dry run") || !strings.Contains(out, "created") {
		t.Errorf("output = %q", out)
	}
	if _, err := os.Stat(filepath.Join(home, "hypr", "huego.conf")); !os.IsNotExist(err) {
		t.Errorf("dry run wrote huego.conf: %v", err)
	}

	out, err = execute(t, "omarchy", "install")
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if *restarts != 0 || !strings.Contains(out, "Restart Waybar") {
		t.Errorf("output = %q, want a hint to restart Waybar", out)
	}
}

func TestOmarchyInstallWithoutConfigFails(t *testing.T) {
	withOmarchyHome(t)
	countWaybarRestarts(t, true)

	_, err := execute(t, "omarchy", "install")
	if err == nil || !strings.Contains(err.Error(), "every file was skipped") {
		t.Fatalf("err = %v, want every file skipped", err)
	}
}
