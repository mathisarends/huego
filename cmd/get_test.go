package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mathisarends/huego/internal/hue"
)

const deskLampID = "11111111-1111-1111-1111-111111111111"

func homeFixture(t *testing.T) http.HandlerFunc {
	mirek := 370
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/clip/v2/resource/light":
			writeJSONFixture(t, w, hue.APIResponse[hue.Light]{Data: []hue.Light{
				{
					ID:       deskLampID,
					Metadata: hue.NamedMetadata{Name: "Desk Lamp", Archetype: "desk_lamp"},
					Owner:    hue.ResourceReference{RID: "device-1", RType: hue.ResourceTypeDevice},
					On:       hue.OnState{On: true},
					Dimming:  &hue.DimmingState{Brightness: 60},
					ColorTemperature: &hue.ColorTemperatureState{
						Mirek: &mirek, MirekValid: true,
						MirekSchema: &hue.MirekSchema{Minimum: 153, Maximum: 454},
					},
				},
				{ID: "light-2", Metadata: hue.NamedMetadata{Name: "Hallway"}, Owner: hue.ResourceReference{RID: "device-2"}},
			}})
		case "/clip/v2/resource/room":
			writeJSONFixture(t, w, hue.APIResponse[hue.Group]{Data: []hue.Group{{
				ID:       "room-1",
				Type:     hue.ResourceTypeRoom,
				Metadata: hue.NamedMetadata{Name: "Office"},
				Children: []hue.ResourceReference{{RID: "device-1", RType: hue.ResourceTypeDevice}},
				Services: []hue.ResourceReference{{RID: "grouped-1", RType: hue.ResourceTypeGroupedLight}},
			}}})
		case "/clip/v2/resource/zone":
			writeJSONFixture(t, w, hue.APIResponse[hue.Group]{})
		case "/clip/v2/resource/grouped_light":
			writeJSONFixture(t, w, hue.APIResponse[hue.GroupedLight]{Data: []hue.GroupedLight{{
				ID: "grouped-1", On: &hue.OnState{On: true}, Dimming: &hue.DimmingState{Brightness: 60},
			}}})
		case "/clip/v2/resource/scene":
			writeJSONFixture(t, w, hue.APIResponse[hue.Scene]{Data: []hue.Scene{
				{ID: "scene-1", Metadata: hue.NamedMetadata{Name: "Relax"}, Group: hue.ResourceReference{RID: "room-1"}},
			}})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}
}

func TestLightListShowsRoomBrightnessAndTemperature(t *testing.T) {
	withClient(t, newFakeClient(t, homeFixture(t)))

	out, err := execute(t, "light", "list")
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	for _, want := range []string{"NAME", "Desk Lamp", "Office", "60%", "2703K"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output = %q, want it to contain %q", out, want)
		}
	}
}

func TestLightListNames(t *testing.T) {
	withClient(t, newFakeClient(t, homeFixture(t)))

	out, err := execute(t, "light", "list", "--names")
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if out != "Desk Lamp\nHallway\n" {
		t.Fatalf("output = %q, want one name per line", out)
	}
}

func TestLightGetJSON(t *testing.T) {
	withClient(t, newFakeClient(t, homeFixture(t)))

	out, err := execute(t, "light", "get", "desk lamp", "--json")
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	var view lightView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if view.ID != deskLampID || view.Room != "Office" || *view.Kelvin != 2703 || view.Capabilities.Temperature[0] != 2203 {
		t.Fatalf("view = %+v, want Desk Lamp in Office at 2703K", view)
	}
}

func TestLightGetText(t *testing.T) {
	withClient(t, newFakeClient(t, homeFixture(t)))

	out, err := execute(t, "light", "get", deskLampID)
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if !strings.Contains(out, "Room:") || !strings.Contains(out, "temperature 2203-6536K") {
		t.Fatalf("output = %q, want room and temperature range", out)
	}
}

func TestRoomGetListsMemberLights(t *testing.T) {
	withClient(t, newFakeClient(t, homeFixture(t)))

	out, err := execute(t, "room", "get", "Office")
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if !strings.Contains(out, "Desk Lamp (on)") || strings.Contains(out, "Hallway") {
		t.Fatalf("output = %q, want only Desk Lamp as member", out)
	}
}

func TestSceneListNamesUsesQualifiedNames(t *testing.T) {
	withClient(t, newFakeClient(t, homeFixture(t)))

	out, err := execute(t, "scene", "list", "--names")
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if out != "Office/Relax\n" {
		t.Fatalf("output = %q, want Office/Relax", out)
	}
}

func TestSceneGet(t *testing.T) {
	withClient(t, newFakeClient(t, homeFixture(t)))

	out, err := execute(t, "scene", "get", "Office/Relax", "--json")
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if !strings.Contains(out, `"group":"Office"`) {
		t.Fatalf("output = %q, want group Office", out)
	}
}

func TestListJSONIsEmptyArrayWhenNothingFound(t *testing.T) {
	withClient(t, newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSONFixture(t, w, hue.APIResponse[hue.Light]{})
	}))

	out, err := execute(t, "light", "list", "--json")
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("output = %q, want []", out)
	}
}
