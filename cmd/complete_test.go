package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/mathisarends/huego/internal/hue"
)

func TestCompletionOffersBridgeNames(t *testing.T) {
	withClient(t, newFakeClient(t, homeFixture(t)))

	tests := []struct {
		args []string
		want string
	}{
		{[]string{"light", "on", ""}, "Desk Lamp"},
		{[]string{"room", "dim", ""}, "Office"},
		{[]string{"scene", "activate", ""}, "Office/Relax"},
		{[]string{"light", "color", "Desk Lamp", ""}, "warm_white"},
		{[]string{"api", ""}, "delete"},
	}
	for _, tt := range tests {
		out, err := execute(t, append([]string{"__complete"}, tt.args...)...)
		if err != nil {
			t.Fatalf("complete %v: %v", tt.args, err)
		}
		if !strings.Contains(out, tt.want+"\n") {
			t.Errorf("complete %v = %q, want %q", tt.args, out, tt.want)
		}
	}
}

func TestCompletionStopsAfterLastArgument(t *testing.T) {
	out, err := execute(t, "__complete", "light", "on", "Desk Lamp", "")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if strings.Contains(out, "Desk Lamp") || !strings.HasPrefix(out, ":4\n") {
		t.Fatalf("complete = %q, want no candidates and no file completion", out)
	}
}

func TestSceneListGroupFilter(t *testing.T) {
	withClient(t, newFakeClient(t, homeFixture(t)))

	out, err := execute(t, "scene", "list", "--group", "office", "--names")
	if err != nil || out != "Office/Relax\n" {
		t.Fatalf("scene list --group office = %q, %v", out, err)
	}

	_, err = execute(t, "scene", "list", "--group", "Attic")
	if report, code := classifyError(err); code != exitNotFound || !strings.Contains(report.Error, "Known: Office") {
		t.Fatalf("err = %v (%s), want not found listing Office", err, report.Code)
	}
}

func TestCompletionCoversEveryCommandGroup(t *testing.T) {
	gets := map[string]string{
		"": allResourcesFixture,
		"/device": `{"errors":[],"data":[{"id":"` + sensorDeviceID + `","metadata":{"name":"Hallway sensor"}},
			{"id":"` + bridgeDeviceID + `","metadata":{"name":"Hue Bridge"}}]}`,
		"/light":               `{"errors":[],"data":[{"id":"` + deskLampID + `","metadata":{"name":"Desk Lamp"}}]}`,
		"/light/" + deskLampID: `{"errors":[],"data":[{"id":"` + deskLampID + `","metadata":{"name":"Desk Lamp"},"effects":{"status":"no_effect","effect_values":["no_effect","candle","fire"]}}]}`,
		"/zone":                `{"errors":[],"data":[{"id":"zone-1","metadata":{"name":"Downstairs"},"children":[],"services":[]}]}`,
	}
	for _, fixtures := range []map[string]string{smartSceneGets, automationGets} {
		for path, body := range fixtures {
			if _, ok := gets[path]; !ok {
				gets[path] = body
			}
		}
	}
	newRoutedBridge(t, gets, nil)

	tests := []struct {
		args []string
		want []string
	}{
		{[]string{"smart-scene", "activate", ""}, []string{"Office/Natural light", "Kitchen/Natural light"}},
		{[]string{"automation", "disable", ""}, []string{"Weekday wake-up"}},
		{[]string{"device", "get", ""}, []string{"Hallway sensor", "Hue Bridge"}},
		{[]string{"sensor", "disable", ""}, []string{"Hallway sensor"}},
		{[]string{"light", "effect", "Desk Lamp", ""}, []string{"none", "candle", "fire"}},
		{[]string{"zone", "on", ""}, []string{"Downstairs"}},
		{[]string{"scene", "list", "--group", ""}, []string{"Office", "Kitchen", "Downstairs"}},
		{[]string{"smart-scene", "list", "--group", ""}, []string{"Office", "Downstairs"}},
		{[]string{"api", "get", ""}, []string{"behavior_instance"}},
	}
	for _, tt := range tests {
		out, err := execute(t, append([]string{"__complete"}, tt.args...)...)
		if err != nil {
			t.Fatalf("complete %v: %v", tt.args, err)
		}
		for _, want := range tt.want {
			if !strings.Contains(out, want+"\n") {
				t.Errorf("complete %v = %q, want %q", tt.args, out, want)
			}
		}
	}
	if out, _ := execute(t, "__complete", "sensor", "disable", ""); strings.Contains(out, "Hue Bridge") {
		t.Errorf("sensor completion offers a device without sensors: %q", out)
	}
}

func TestCompletionWithoutBridgeReportsAnError(t *testing.T) {
	orig := newClient
	t.Cleanup(func() { newClient = orig })
	newClient = func() (*hue.Client, error) { return nil, errors.New("no bridge configured") }

	for _, args := range [][]string{{"light", "on", ""}, {"light", "effect", "Desk Lamp", ""}} {
		out, err := execute(t, append([]string{"__complete"}, args...)...)
		if err != nil {
			t.Fatalf("complete %v: %v", args, err)
		}
		if !strings.Contains(out, ":1\n") {
			t.Errorf("complete %v = %q, want the error directive", args, out)
		}
	}
}
