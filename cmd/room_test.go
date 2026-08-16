package cmd

import (
	"net/http"
	"strings"
	"testing"

	"github.com/mathisarends/huego/internal/hue"
)

func TestRoomListCommand(t *testing.T) {
	client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/clip/v2/resource/room":
			writeJSONFixture(t, w, hue.APIResponse[hue.Group]{Data: []hue.Group{{
				ID:       "room-1",
				Metadata: hue.NamedMetadata{Name: "Kitchen"},
				Services: []hue.ResourceReference{{RID: "grouped-1", RType: hue.ResourceTypeGroupedLight}},
			}}})
		case "/clip/v2/resource/grouped_light":
			writeJSONFixture(t, w, hue.APIResponse[hue.GroupedLight]{Data: []hue.GroupedLight{{
				ID: "grouped-1", On: &hue.OnState{On: true},
			}}})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	withClient(t, client)

	out, err := execute(t, "room", "list")
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if !strings.Contains(out, "Kitchen") || !strings.Contains(out, "(on)") {
		t.Fatalf("output = %q, want it to mention Kitchen and on", out)
	}
}

func TestRoomOnCommandByName(t *testing.T) {
	var putPath string
	var gotBody hue.LightUpdate
	client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/clip/v2/resource/room":
			writeJSONFixture(t, w, hue.APIResponse[hue.Group]{Data: []hue.Group{
				{
					ID:       "room-1",
					Metadata: hue.NamedMetadata{Name: "Kitchen"},
					Services: []hue.ResourceReference{{RID: "grouped-1", RType: hue.ResourceTypeGroupedLight}},
				},
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/clip/v2/resource/room/room-1":
			writeJSONFixture(t, w, hue.APIResponse[hue.Group]{Data: []hue.Group{
				{
					ID:       "room-1",
					Services: []hue.ResourceReference{{RID: "grouped-1", RType: hue.ResourceTypeGroupedLight}},
				},
			}})
		case r.Method == http.MethodPut:
			putPath = r.URL.Path
			decodeJSONFixture(t, r, &gotBody)
			writeJSONFixture(t, w, hue.APIResponse[hue.ResourceIdentifier]{
				Data: []hue.ResourceIdentifier{{RID: "grouped-1", RType: hue.ResourceTypeGroupedLight}},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	withClient(t, client)

	out, err := execute(t, "room", "on", "Kitchen")
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if !strings.Contains(out, `Updated "Kitchen" (1 resource(s)).`) {
		t.Fatalf("output = %q, want update confirmation", out)
	}
	if putPath != "/clip/v2/resource/grouped_light/grouped-1" {
		t.Fatalf("PUT path = %q, want grouped_light/grouped-1", putPath)
	}
	if gotBody.On == nil || !gotBody.On.On {
		t.Fatalf("request body On = %+v, want on=true", gotBody.On)
	}
}

func TestRoomToggleCommand(t *testing.T) {
	const roomID = "01234567-89ab-cdef-0123-456789abcdef"
	var gotBody hue.LightUpdate
	client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/clip/v2/resource/room/"+roomID:
			writeJSONFixture(t, w, hue.APIResponse[hue.Group]{Data: []hue.Group{
				{
					ID:       roomID,
					Services: []hue.ResourceReference{{RID: "grouped-1", RType: hue.ResourceTypeGroupedLight}},
				},
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/clip/v2/resource/grouped_light/grouped-1":
			writeJSONFixture(t, w, hue.APIResponse[hue.GroupedLight]{Data: []hue.GroupedLight{
				{ID: "grouped-1", On: &hue.OnState{On: false}},
			}})
		case r.Method == http.MethodPut:
			decodeJSONFixture(t, r, &gotBody)
			writeJSONFixture(t, w, hue.APIResponse[hue.ResourceIdentifier]{
				Data: []hue.ResourceIdentifier{{RID: "grouped-1", RType: hue.ResourceTypeGroupedLight}},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	withClient(t, client)

	out, err := execute(t, "room", "toggle", roomID)
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if !strings.Contains(out, `Updated "`+roomID+`" (1 resource(s)).`) {
		t.Fatalf("output = %q, want update confirmation", out)
	}
	if gotBody.On == nil || !gotBody.On.On {
		t.Fatalf("request body On = %+v, want toggle from off to on=true", gotBody.On)
	}
}
