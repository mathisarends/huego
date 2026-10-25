package cmd

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

const (
	officeRelaxID  = "eeeeeeee-0000-0000-0000-000000000001"
	officeFocusID  = "eeeeeeee-0000-0000-0000-000000000002"
	kitchenRelaxID = "eeeeeeee-0000-0000-0000-000000000003"
)

var pickGets = map[string]string{
	"/scene": `{"errors":[],"data":[
		{"id":"` + officeRelaxID + `","metadata":{"name":"Relax"},"group":{"rid":"` + officeRoomID + `","rtype":"room"}},
		{"id":"` + officeFocusID + `","metadata":{"name":"Focus"},"group":{"rid":"` + officeRoomID + `","rtype":"room"}},
		{"id":"` + kitchenRelaxID + `","metadata":{"name":"Relax"},"group":{"rid":"` + kitchenRoomID + `","rtype":"room"}}]}`,
	"/room": smartSceneGets["/room"],
	"/zone": smartSceneGets["/zone"],
}

type pickerCall struct {
	argv    []string
	choices string
}

// fakePicker makes only the named programs appear on PATH and answers every
// picker run with chosen.
func fakePicker(t *testing.T, chosen string, onPath ...string) *pickerCall {
	t.Helper()
	t.Setenv("HUEGO_PICKER", "")
	call := &pickerCall{}
	origLookPath, origRun := lookPath, runPicker
	t.Cleanup(func() { lookPath, runPicker = origLookPath, origRun })
	lookPath = func(name string) (string, error) {
		if slices.Contains(onPath, name) {
			return "/usr/bin/" + name, nil
		}
		return "", errors.New("not found")
	}
	runPicker = func(_ context.Context, argv []string, choices string) (string, error) {
		call.argv, call.choices = argv, choices
		return chosen, nil
	}
	return call
}

func TestScenePickActivatesTheChosenScene(t *testing.T) {
	var puts []string
	newRoutedBridge(t, pickGets, &puts)
	call := fakePicker(t, "Office/Relax", "fzf", "rofi")

	out, err := execute(t, "scene", "pick")
	if err != nil {
		t.Fatalf("scene pick: %v\n%s", err, out)
	}
	if call.argv[0] != "rofi" {
		t.Errorf("picker = %v, want rofi, the first known picker on PATH", call.argv)
	}
	if want := "Office/Relax\nOffice/Focus\nKitchen/Relax\n"; call.choices != want {
		t.Errorf("choices = %q, want %q", call.choices, want)
	}
	if len(puts) != 1 || !strings.HasPrefix(puts[0], "/scene/"+officeRelaxID+" ") || !strings.Contains(puts[0], `"active"`) {
		t.Errorf("puts = %v, want one recall of Office/Relax", puts)
	}
	if !strings.Contains(out, "Office/Relax") {
		t.Errorf("output = %q, want it to name the scene", out)
	}
}

func TestScenePickDismissedActivatesNothing(t *testing.T) {
	var puts []string
	newRoutedBridge(t, pickGets, &puts)
	fakePicker(t, "", "walker")

	out, err := execute(t, "scene", "pick")
	if err != nil {
		t.Fatalf("scene pick: %v", err)
	}
	if len(puts) != 0 || out != "" {
		t.Errorf("puts = %v, output = %q, want nothing", puts, out)
	}
}

func TestScenePickUsesConfiguredPickerAndGroup(t *testing.T) {
	var puts []string
	newRoutedBridge(t, pickGets, &puts)
	call := fakePicker(t, "Kitchen/Relax", "walker")
	t.Setenv("HUEGO_PICKER", "env-menu")

	if _, err := execute(t, "scene", "pick", "--group", "Kitchen", "--picker", "my-menu --flag"); err != nil {
		t.Fatalf("scene pick: %v", err)
	}
	if !slices.Equal(call.argv, []string{"my-menu", "--flag"}) {
		t.Errorf("picker = %v, want --picker to win over HUEGO_PICKER", call.argv)
	}
	if call.choices != "Kitchen/Relax\n" {
		t.Errorf("choices = %q, want only Kitchen's scenes", call.choices)
	}
	if len(puts) != 1 || !strings.HasPrefix(puts[0], "/scene/"+kitchenRelaxID+" ") {
		t.Errorf("puts = %v", puts)
	}

	if _, err := execute(t, "scene", "pick"); err != nil {
		t.Fatalf("scene pick: %v", err)
	}
	if !slices.Equal(call.argv, []string{"env-menu"}) {
		t.Errorf("picker = %v, want HUEGO_PICKER", call.argv)
	}
}

func TestScenePickNotifiesErrors(t *testing.T) {
	fakePicker(t, "")
	var notified []string
	orig := notify
	t.Cleanup(func() { notify = orig })
	notify = func(message string) { notified = append(notified, message) }

	_, err := execute(t, "scene", "pick", "--notify")
	if err == nil || !strings.Contains(err.Error(), "no picker found") {
		t.Fatalf("err = %v, want no picker found", err)
	}
	if len(notified) != 1 || notified[0] != err.Error() {
		t.Errorf("notified = %v, want the error", notified)
	}

	if _, err := execute(t, "scene", "pick"); err == nil {
		t.Fatal("want error")
	}
	if len(notified) != 1 {
		t.Errorf("notified without --notify: %v", notified)
	}
}
