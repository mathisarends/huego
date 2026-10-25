package omarchy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const waybarConfig = `{
  "reload_style_on_change": true,
  "layer": "top",
  // Omarchy keeps comments in here.
  "modules-left": ["custom/omarchy", "hyprland/workspaces"],
  "modules-center": ["clock"],
  "modules-right": [
    "group/tray-expander",
    "bluetooth",
    "network"
  ],
  "clock": { "format": "{:%A %H:%M}" }
}
`

const waybarStyle = `@import "../omarchy/current/theme/waybar.css";

* {
  font-size: 12px;
}
`

const hyprlandConfig = `source = ~/.local/share/omarchy/default/hypr/bindings.conf
source = ~/.config/hypr/bindings.conf
`

func omarchyHome(t *testing.T) Options {
	t.Helper()
	home := t.TempDir()
	for path, content := range map[string]string{
		"hypr/hyprland.conf":  hyprlandConfig,
		"waybar/config.jsonc": waybarConfig,
		"waybar/style.css":    waybarStyle,
	} {
		full := filepath.Join(home, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return Options{ConfigHome: home}
}

func read(t *testing.T, o Options, parts ...string) string {
	t.Helper()
	content, err := os.ReadFile(o.path(parts...))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func actions(changes []Change) map[string]Action {
	got := map[string]Action{}
	for _, c := range changes {
		got[filepath.Base(c.File)] = c.Action
	}
	return got
}

// jsoncComment strips // comments so the result can be checked with the
// standard JSON decoder.
var jsoncComment = regexp.MustCompile(`(?m)^\s*//.*$|\s//\s.*$`)

func TestInstallAddsModuleBindingsAndStyle(t *testing.T) {
	o := omarchyHome(t)

	changes, err := Install(o)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	want := map[string]Action{"huego.conf": Created, "hyprland.conf": Updated, "config.jsonc": Updated, "style.css": Updated}
	if got := actions(changes); len(got) != len(want) {
		t.Fatalf("changes = %+v", changes)
	} else {
		for file, action := range want {
			if got[file] != action {
				t.Errorf("%s: %s, want %s", file, got[file], action)
			}
		}
	}
	if !HasWaybarChanges(changes) {
		t.Error("HasWaybarChanges = false")
	}

	config := read(t, o, "waybar", "config.jsonc")
	var bar struct {
		ModulesRight []string                   `json:"modules-right"`
		Hue          map[string]json.RawMessage `json:"custom/hue"`
	}
	if err := json.Unmarshal([]byte(jsoncComment.ReplaceAllString(config, "")), &bar); err != nil {
		t.Fatalf("installed config is not valid JSONC: %v\n%s", err, config)
	}
	if bar.ModulesRight[0] != "custom/hue" || len(bar.ModulesRight) != 4 || string(bar.Hue["exec"]) != `"huego waybar --watch"` {
		t.Fatalf("bar = %+v", bar)
	}
	if !strings.Contains(config, "// Omarchy keeps comments in here.") {
		t.Error("user comments were lost")
	}

	bindings := read(t, o, "hypr", "huego.conf")
	if !strings.Contains(bindings, "bindd = SUPER CTRL ALT, H, Toggle lights, exec, huego home toggle") {
		t.Errorf("bindings use the wrong modifier:\n%s", bindings)
	}
	if hypr := read(t, o, "hypr", "hyprland.conf"); !strings.HasPrefix(hypr, hyprlandConfig) ||
		!strings.Contains(hypr, "source = "+filepath.ToSlash(o.path("hypr", "huego.conf"))) {
		t.Errorf("hyprland.conf:\n%s", hypr)
	}
	if style := read(t, o, "waybar", "style.css"); !strings.HasPrefix(style, waybarStyle) || !strings.Contains(style, "#custom-hue.off") {
		t.Errorf("style.css:\n%s", style)
	}
	if backup := read(t, o, "waybar", "config.jsonc.huego.bak"); backup != waybarConfig {
		t.Errorf("backup = %q, want the original", backup)
	}
}

func TestInstallTwiceChangesNothing(t *testing.T) {
	o := omarchyHome(t)
	if _, err := Install(o); err != nil {
		t.Fatal(err)
	}
	first := read(t, o, "waybar", "config.jsonc")

	changes, err := Install(o)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range changes {
		if c.Action != Unchanged {
			t.Errorf("%s: %s, want unchanged", c.File, c.Action)
		}
	}
	if HasWaybarChanges(changes) {
		t.Error("HasWaybarChanges = true for a no-op install")
	}
	if read(t, o, "waybar", "config.jsonc") != first {
		t.Error("second install changed config.jsonc")
	}
}

func TestInstallWithOtherModifierUpdatesBindings(t *testing.T) {
	o := omarchyHome(t)
	if _, err := Install(o); err != nil {
		t.Fatal(err)
	}
	o.Modifier = "SUPER SHIFT"
	changes, err := Install(o)
	if err != nil {
		t.Fatal(err)
	}
	if actions(changes)["huego.conf"] != Updated || !strings.Contains(read(t, o, "hypr", "huego.conf"), "bindd = SUPER SHIFT, H,") {
		t.Fatalf("changes = %+v", changes)
	}
}

func TestUninstallRestoresOriginals(t *testing.T) {
	o := omarchyHome(t)
	if _, err := Install(o); err != nil {
		t.Fatal(err)
	}

	changes, err := Uninstall(o)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range changes {
		if c.Action != Removed {
			t.Errorf("%s: %s, want removed", c.File, c.Action)
		}
	}
	for path, want := range map[string]string{"hypr/hyprland.conf": hyprlandConfig, "waybar/config.jsonc": waybarConfig, "waybar/style.css": waybarStyle} {
		if got := read(t, o, strings.Split(path, "/")...); got != want {
			t.Errorf("%s = %q, want the original %q", path, got, want)
		}
	}
	if _, err := os.Stat(o.path("hypr", "huego.conf")); !os.IsNotExist(err) {
		t.Errorf("huego.conf still exists: %v", err)
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	o := omarchyHome(t)
	o.DryRun = true

	changes, err := Install(o)
	if err != nil {
		t.Fatal(err)
	}
	if actions(changes)["config.jsonc"] != Updated {
		t.Fatalf("changes = %+v, want the planned update", changes)
	}
	if read(t, o, "waybar", "config.jsonc") != waybarConfig {
		t.Error("dry run changed config.jsonc")
	}
	if _, err := os.Stat(o.path("hypr", "huego.conf")); !os.IsNotExist(err) {
		t.Error("dry run created huego.conf")
	}
}

func TestInstallSkipsMissingAndHandWrittenConfigs(t *testing.T) {
	o := Options{ConfigHome: t.TempDir()}
	changes, err := Install(o)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range changes {
		if c.Action != Skipped {
			t.Errorf("%s: %s, want skipped", c.File, c.Action)
		}
	}

	o = omarchyHome(t)
	custom := strings.Replace(waybarConfig, `"bluetooth",`, `"bluetooth", "custom/hue",`, 1)
	if err := os.WriteFile(o.path("waybar", "config.jsonc"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}
	changes, err = Install(o)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range changes {
		if filepath.Base(c.File) == "config.jsonc" && (c.Action != Skipped || !strings.Contains(c.Note, "by hand")) {
			t.Errorf("config.jsonc: %+v, want skipped as hand-configured", c)
		}
	}
}

func TestAddWaybarModuleHandlesInlineAndEmptyLists(t *testing.T) {
	module, err := template("waybar-module.jsonc")
	if err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		config string
		count  int
	}{
		"inline": {"{\n  \"modules-right\": [\"clock\"]\n}\n", 1},
		"empty":  {"{\n  \"modules-right\": []\n}\n", 0},
	} {
		t.Run(name, func(t *testing.T) {
			edited, err := addWaybarModule(tc.config, module)
			if err != nil {
				t.Fatal(err)
			}
			var bar struct {
				ModulesRight []string `json:"modules-right"`
			}
			if err := json.Unmarshal([]byte(jsoncComment.ReplaceAllString(edited, "")), &bar); err != nil {
				t.Fatalf("edited config is not valid JSON: %v\n%s", err, edited)
			}
			if len(bar.ModulesRight) != tc.count+1 || bar.ModulesRight[0] != "custom/hue" {
				t.Errorf("modules-right = %v", bar.ModulesRight)
			}
			if restored := stripManaged(edited, slashBegin, slashEnd); restored != tc.config {
				var after struct {
					ModulesRight []string `json:"modules-right"`
				}
				if err := json.Unmarshal([]byte(restored), &after); err != nil || len(after.ModulesRight) != tc.count {
					t.Errorf("stripManaged left %q", restored)
				}
			}
		})
	}

	if _, err := addWaybarModule("{\n  \"layer\": \"top\"\n}\n", module); err == nil {
		t.Error("config without modules-right: want error")
	}
}

func TestInstallReportsUnwritableFiles(t *testing.T) {
	o := omarchyHome(t)
	if err := os.MkdirAll(o.path("hypr", "huego.conf"), 0o755); err != nil {
		t.Fatal(err)
	}

	changes, err := Install(o)
	if err == nil {
		t.Fatal("Install with a directory in place of huego.conf: want error")
	}
	if got := actions(changes); got["huego.conf"] != Skipped {
		t.Errorf("changes = %+v", changes)
	}
	if strings.Contains(read(t, o, "waybar", "config.jsonc"), "custom/hue") {
		t.Error("Install went on after the error")
	}
}

func TestUninstallWithoutInstallChangesNothing(t *testing.T) {
	o := omarchyHome(t)

	changes, err := Uninstall(o)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range changes {
		if c.Action != Unchanged {
			t.Errorf("%+v, want unchanged", c)
		}
	}
	if HasWaybarChanges(changes) {
		t.Error("HasWaybarChanges = true")
	}
}

func TestAppendBlockToEmptyFile(t *testing.T) {
	if got := appendBlock("\n", "block\n"); got != "block\n" {
		t.Errorf("appendBlock = %q", got)
	}
}
