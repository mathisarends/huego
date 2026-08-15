package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mathisarends/huego/internal/hue"
)

func TestLightListCommand(t *testing.T) {
	client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSONFixture(t, w, hue.APIResponse[hue.Light]{Data: []hue.Light{
			{ID: "1", Metadata: hue.NamedMetadata{Name: "Desk Lamp"}, On: hue.OnState{On: true}},
		}})
	})
	withClient(t, client)

	out, err := execute(t, "light", "list")
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if !strings.Contains(out, "Desk Lamp") || !strings.Contains(out, "on") {
		t.Fatalf("output = %q, want it to mention Desk Lamp and on", out)
	}
}

func TestLightListCommandJSON(t *testing.T) {
	client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSONFixture(t, w, hue.APIResponse[hue.Light]{Data: []hue.Light{
			{ID: "1", Metadata: hue.NamedMetadata{Name: "Desk Lamp"}, On: hue.OnState{On: true}},
		}})
	})
	withClient(t, client)

	out, err := execute(t, "light", "list", "--json")
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if !strings.Contains(out, `"id":"1"`) {
		t.Fatalf("output = %q, want JSON with id 1", out)
	}
}

func TestLightOnCommandByName(t *testing.T) {
	var gotBody hue.LightUpdate
	client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/clip/v2/resource/light":
			writeJSONFixture(t, w, hue.APIResponse[hue.Light]{Data: []hue.Light{
				{ID: "abc", Metadata: hue.NamedMetadata{Name: "Desk Lamp"}},
			}})
		case r.Method == http.MethodPut:
			decodeJSONFixture(t, r, &gotBody)
			writeJSONFixture(t, w, hue.APIResponse[hue.ResourceIdentifier]{
				Data: []hue.ResourceIdentifier{{RID: "abc", RType: hue.ResourceTypeLight}},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	withClient(t, client)

	out, err := execute(t, "light", "on", "Desk Lamp", "--brightness", "50")
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if !strings.Contains(out, "Updated 1 resource(s).") {
		t.Fatalf("output = %q, want update confirmation", out)
	}
	if gotBody.On == nil || !gotBody.On.On {
		t.Fatalf("request body On = %+v, want on=true", gotBody.On)
	}
	if gotBody.Dimming == nil || gotBody.Dimming.Brightness != 50 {
		t.Fatalf("request body Dimming = %+v, want brightness 50", gotBody.Dimming)
	}
}

func decodeJSONFixture(t *testing.T, r *http.Request, v any) {
	t.Helper()
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
}
