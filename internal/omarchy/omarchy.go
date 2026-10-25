// Package omarchy adds huego's Hyprland keybindings and Waybar module to an
// Omarchy user's config. Every change sits between huego markers, so
// installing again updates it in place and Uninstall removes exactly it.
package omarchy

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed files
var files embed.FS

const DefaultModifier = "SUPER CTRL ALT"

type Action string

const (
	Created   Action = "created"
	Updated   Action = "updated"
	Unchanged Action = "unchanged"
	Removed   Action = "removed"
	Skipped   Action = "skipped"
)

type Change struct {
	File   string `json:"file"`
	Action Action `json:"action"`
	Note   string `json:"note,omitempty"`
}

type Options struct {
	// ConfigHome is the directory holding hypr/ and waybar/, usually ~/.config.
	ConfigHome string
	// Modifier is the Hyprland modifier for every keybinding, such as "SUPER CTRL ALT".
	Modifier string
	DryRun   bool
}

const (
	hashBegin  = "# huego:begin"
	hashEnd    = "# huego:end"
	slashBegin = "// huego:begin"
	slashEnd   = "// huego:end"
	cssBegin   = "/* huego:begin */"
	cssEnd     = "/* huego:end */"
	lineMarker = "// huego"
	moduleName = `"custom/hue"`
)

func (o Options) path(parts ...string) string {
	return filepath.Join(append([]string{o.ConfigHome}, parts...)...)
}

func Install(o Options) ([]Change, error) {
	if o.Modifier == "" {
		o.Modifier = DefaultModifier
	}
	bindings, err := template("hyprland.conf")
	if err != nil {
		return nil, err
	}
	bindingsPath := o.path("hypr", "huego.conf")
	bindings = strings.ReplaceAll(bindings, "{{MOD}}", o.Modifier)

	var changes []Change
	add := func(change Change, err error) error {
		changes = append(changes, change)
		return err
	}

	if _, err := os.Stat(o.path("hypr", "hyprland.conf")); err != nil {
		changes = append(changes, Change{File: o.path("hypr", "hyprland.conf"), Action: Skipped, Note: "Hyprland config not found"})
	} else {
		if err := add(writeFile(o, bindingsPath, bindings)); err != nil {
			return changes, err
		}
		source := hashBegin + "\nsource = " + filepath.ToSlash(bindingsPath) + "\n" + hashEnd + "\n"
		if err := add(editFile(o, o.path("hypr", "hyprland.conf"), func(content string) (string, error) {
			return appendBlock(stripManaged(content, hashBegin, hashEnd), source), nil
		})); err != nil {
			return changes, err
		}
	}

	module, err := template("waybar-module.jsonc")
	if err != nil {
		return changes, err
	}
	if err := add(editFile(o, o.path("waybar", "config.jsonc"), func(content string) (string, error) {
		return addWaybarModule(stripManaged(content, slashBegin, slashEnd), module)
	})); err != nil {
		return changes, err
	}

	style, err := template("waybar-style.css")
	if err != nil {
		return changes, err
	}
	if err := add(editFile(o, o.path("waybar", "style.css"), func(content string) (string, error) {
		return appendBlock(stripManaged(content, cssBegin, cssEnd), cssBegin+"\n"+style+cssEnd+"\n"), nil
	})); err != nil {
		return changes, err
	}
	return changes, nil
}

func Uninstall(o Options) ([]Change, error) {
	var changes []Change
	edits := []struct {
		path       string
		begin, end string
		appended   bool
	}{
		{o.path("hypr", "hyprland.conf"), hashBegin, hashEnd, true},
		{o.path("waybar", "config.jsonc"), slashBegin, slashEnd, false},
		{o.path("waybar", "style.css"), cssBegin, cssEnd, true},
	}
	for _, e := range edits {
		change, err := editFile(o, e.path, func(content string) (string, error) {
			stripped := stripManaged(content, e.begin, e.end)
			if e.appended && stripped != content {
				stripped = strings.TrimRight(stripped, "\n") + "\n"
			}
			return stripped, nil
		})
		if change.Action == Updated {
			change.Action = Removed
			change.Note = "huego section removed"
		}
		changes = append(changes, change)
		if err != nil {
			return changes, err
		}
	}

	bindings := o.path("hypr", "huego.conf")
	switch _, err := os.Stat(bindings); {
	case errors.Is(err, fs.ErrNotExist):
		changes = append(changes, Change{File: bindings, Action: Unchanged})
	case err != nil:
		return changes, err
	default:
		if !o.DryRun {
			if err := os.Remove(bindings); err != nil {
				return changes, err
			}
		}
		changes = append(changes, Change{File: bindings, Action: Removed})
	}
	return changes, nil
}

// HasWaybarChanges reports whether changes touched Waybar's config, which
// Waybar only reads at startup.
func HasWaybarChanges(changes []Change) bool {
	for _, c := range changes {
		if (c.Action == Created || c.Action == Updated || c.Action == Removed) && filepath.Base(filepath.Dir(c.File)) == "waybar" {
			return true
		}
	}
	return false
}

func template(name string) (string, error) {
	content, err := files.ReadFile("files/" + name)
	return string(content), err
}

func writeFile(o Options, path, content string) (Change, error) {
	existing, err := os.ReadFile(path)
	switch {
	case err == nil && string(existing) == content:
		return Change{File: path, Action: Unchanged}, nil
	case err == nil:
		return Change{File: path, Action: Updated}, write(o, path, content, 0o644)
	case errors.Is(err, fs.ErrNotExist):
		return Change{File: path, Action: Created}, write(o, path, content, 0o644)
	default:
		return Change{File: path, Action: Skipped}, err
	}
}

// editFile rewrites an existing config file with edit. The first time huego
// changes a file it keeps the original next to it as <file>.huego.bak.
func editFile(o Options, path string, edit func(string) (string, error)) (Change, error) {
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Change{File: path, Action: Skipped, Note: "file not found"}, nil
	}
	if err != nil {
		return Change{File: path, Action: Skipped}, err
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return Change{File: path, Action: Skipped}, err
	}
	edited, err := edit(string(original))
	if err != nil {
		return Change{File: path, Action: Skipped, Note: err.Error()}, nil
	}
	if edited == string(original) {
		return Change{File: path, Action: Unchanged}, nil
	}

	change := Change{File: path, Action: Updated}
	backup := path + ".huego.bak"
	if _, err := os.Stat(backup); errors.Is(err, fs.ErrNotExist) {
		change.Note = "original saved as " + filepath.Base(backup)
		if err := write(o, backup, string(original), info.Mode().Perm()); err != nil {
			return change, err
		}
	}
	return change, write(o, path, edited, info.Mode().Perm())
}

func write(o Options, path, content string, perm fs.FileMode) error {
	if o.DryRun {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), perm)
}

// stripManaged removes the lines from begin to end and the single lines
// huego marks with a trailing "// huego".
func stripManaged(content, begin, end string) string {
	lines := strings.SplitAfter(content, "\n")
	kept := lines[:0]
	inside := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == begin:
			inside = true
		case trimmed == end:
			inside = false
		case inside:
		case strings.HasPrefix(trimmed, moduleName) && strings.HasSuffix(trimmed, lineMarker):
		default:
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "")
}

func appendBlock(content, block string) string {
	if strings.TrimSpace(content) == "" {
		return block
	}
	return strings.TrimRight(content, "\n") + "\n\n" + block
}

var (
	firstObject  = regexp.MustCompile(`(?m)^[ \t]*\{[^\n]*\n`)
	modulesRight = regexp.MustCompile(`"modules-right"\s*:\s*\[`)
)

// addWaybarModule defines custom/hue at the top of the bar's settings and
// puts it first in modules-right. It edits the text rather than decoding
// it so the user's comments and layout survive.
func addWaybarModule(content, module string) (string, error) {
	if strings.Contains(content, moduleName) {
		return "", fmt.Errorf("%s is already configured by hand", moduleName)
	}
	object := firstObject.FindStringIndex(content)
	list := modulesRight.FindStringIndex(content)
	if object == nil || list == nil {
		return "", errors.New(`no "modules-right" list found; add custom/hue by hand`)
	}

	entry := "\n    " + moduleName + ", " + lineMarker
	rest := strings.TrimLeft(content[list[1]:], " \t")
	switch {
	case strings.HasPrefix(rest, "]"):
		entry = "\n    " + moduleName + " " + lineMarker + "\n"
	case !strings.HasPrefix(rest, "\n") && !strings.HasPrefix(rest, "\r\n"):
		entry += "\n    "
	}

	var indented strings.Builder
	for _, line := range strings.SplitAfter(slashBegin+"\n"+module+slashEnd+"\n", "\n") {
		if line != "" {
			indented.WriteString("  ")
			indented.WriteString(line)
		}
	}

	return content[:object[1]] + indented.String() + content[object[1]:list[1]] + entry + content[list[1]:], nil
}
