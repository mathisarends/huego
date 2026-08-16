package cmd

import (
	"encoding/json"
	"errors"
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
	if !strings.Contains(out, `Updated "Desk Lamp" (1 resource(s)).`) {
		t.Fatalf("output = %q, want named update confirmation", out)
	}
	if gotBody.On == nil || !gotBody.On.On {
		t.Fatalf("request body On = %+v, want on=true", gotBody.On)
	}
	if gotBody.Dimming == nil || gotBody.Dimming.Brightness != 50 {
		t.Fatalf("request body Dimming = %+v, want brightness 50", gotBody.Dimming)
	}
}

func TestLightOnCommandReportsBridgeErrors(t *testing.T) {
	client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSONFixture(t, w, hue.APIResponse[hue.ResourceIdentifier]{
			Errors: []struct {
				Description string `json:"description"`
			}{{Description: "light has communication issues"}},
		})
	})
	withClient(t, client)

	_, err := execute(t, "light", "on", testLightUUID)
	if err == nil || !strings.Contains(err.Error(), "communication issues") || !strings.Contains(err.Error(), testLightUUID) {
		t.Fatalf("execute() error = %v, want target and bridge error", err)
	}
}

func TestLightOnCommandJSONWritesResponseAndReportsBridgeErrors(t *testing.T) {
	client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSONFixture(t, w, hue.APIResponse[hue.ResourceIdentifier]{
			Errors: []struct {
				Description string `json:"description"`
			}{{Description: "light has communication issues"}},
		})
	})
	withClient(t, client)

	out, err := execute(t, "light", "on", testLightUUID, "--json")
	if err == nil || !strings.Contains(err.Error(), "communication issues") {
		t.Fatalf("execute() error = %v, want bridge error", err)
	}
	if !strings.Contains(out, `"errors":[{"description":"light has communication issues"}]`) {
		t.Fatalf("output = %q, want JSON bridge response", out)
	}
}

func TestLightOnCommandClientError(t *testing.T) {
	orig := newClient
	t.Cleanup(func() { newClient = orig })
	wantErr := "no bridge configured"
	newClient = func() (*hue.Client, error) { return nil, errors.New(wantErr) }

	_, err := execute(t, "light", "on", testLightUUID)
	if err == nil || !strings.Contains(err.Error(), wantErr) {
		t.Fatalf("execute() error = %v, want it to mention %q", err, wantErr)
	}
}

func TestLightListCommandReportsEmpty(t *testing.T) {
	client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSONFixture(t, w, hue.APIResponse[hue.Light]{Data: []hue.Light{}})
	})
	withClient(t, client)

	out, err := execute(t, "light", "list")
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if !strings.Contains(out, "No results.") {
		t.Fatalf("output = %q, want it to report no results", out)
	}
}

const testLightUUID = "01234567-89ab-cdef-0123-456789abcdef"

func TestLightColorCommand(t *testing.T) {
	var gotBody hue.LightUpdate
	client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			decodeJSONFixture(t, r, &gotBody)
			writeJSONFixture(t, w, hue.APIResponse[hue.ResourceIdentifier]{
				Data: []hue.ResourceIdentifier{{RID: testLightUUID, RType: hue.ResourceTypeLight}},
			})
		}
	})
	withClient(t, client)

	if _, err := execute(t, "light", "color", testLightUUID, "#ff0000"); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if gotBody.Color == nil || gotBody.Color.XY == nil {
		t.Fatalf("request body Color = %+v, want an XY value", gotBody.Color)
	}
}

func TestLightColorCommandInvalidHex(t *testing.T) {
	client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("bridge should not be called for an invalid color")
	})
	withClient(t, client)

	if _, err := execute(t, "light", "color", testLightUUID, "not-a-color"); err == nil {
		t.Fatal("execute() error = nil, want error")
	}
}

func TestLightTemperatureCommand(t *testing.T) {
	var gotBody hue.LightUpdate
	client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			decodeJSONFixture(t, r, &gotBody)
			writeJSONFixture(t, w, hue.APIResponse[hue.ResourceIdentifier]{
				Data: []hue.ResourceIdentifier{{RID: testLightUUID, RType: hue.ResourceTypeLight}},
			})
		}
	})
	withClient(t, client)

	if _, err := execute(t, "light", "temperature", testLightUUID, "2700"); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if gotBody.ColorTemperature == nil || *gotBody.ColorTemperature.Mirek != 370 {
		t.Fatalf("request body ColorTemperature = %+v, want mirek 370", gotBody.ColorTemperature)
	}
}

func TestLightTemperatureCommandInvalidKelvin(t *testing.T) {
	client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("bridge should not be called for an invalid kelvin value")
	})
	withClient(t, client)

	if _, err := execute(t, "light", "temperature", testLightUUID, "not-a-number"); err == nil {
		t.Fatal("execute() error = nil, want error")
	}
}

func TestLightIdentifyCommand(t *testing.T) {
	var gotBody hue.LightUpdate
	client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			decodeJSONFixture(t, r, &gotBody)
			writeJSONFixture(t, w, hue.APIResponse[hue.ResourceIdentifier]{
				Data: []hue.ResourceIdentifier{{RID: testLightUUID, RType: hue.ResourceTypeLight}},
			})
		}
	})
	withClient(t, client)

	if _, err := execute(t, "light", "identify", testLightUUID); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if gotBody.Alert == nil {
		t.Fatalf("request body Alert = %+v, want a value", gotBody.Alert)
	}
}

func TestLightBrightnessCommandZeroTurnsOff(t *testing.T) {
	var gotBody hue.LightUpdate
	client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			decodeJSONFixture(t, r, &gotBody)
			writeJSONFixture(t, w, hue.APIResponse[hue.ResourceIdentifier]{
				Data: []hue.ResourceIdentifier{{RID: testLightUUID, RType: hue.ResourceTypeLight}},
			})
		}
	})
	withClient(t, client)

	if _, err := execute(t, "light", "brightness", testLightUUID, "0"); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if gotBody.On == nil || gotBody.On.On {
		t.Fatalf("request body On = %+v, want on=false", gotBody.On)
	}
}

func TestLightBrightnessCommandInvalidPercent(t *testing.T) {
	client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("bridge should not be called for an invalid percent")
	})
	withClient(t, client)

	if _, err := execute(t, "light", "brightness", testLightUUID, "not-a-number"); err == nil {
		t.Fatal("execute() error = nil, want error")
	}
}

func decodeJSONFixture(t *testing.T, r *http.Request, v any) {
	t.Helper()
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
}
