package cmd

import (
	"net/http"
	"strings"
	"testing"

	"github.com/mathisarends/huego/internal/hue"
)

func TestZoneListCommand(t *testing.T) {
	client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSONFixture(t, w, hue.APIResponse[hue.Group]{Data: []hue.Group{
			{ID: "zone-1", Metadata: hue.NamedMetadata{Name: "Upstairs"}},
		}})
	})
	withClient(t, client)

	out, err := execute(t, "zone", "list")
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if !strings.Contains(out, "Upstairs") {
		t.Fatalf("output = %q, want it to mention Upstairs", out)
	}
}

func TestZoneOffCommandByName(t *testing.T) {
	var putPath string
	var gotBody hue.LightUpdate
	client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/clip/v2/resource/zone":
			writeJSONFixture(t, w, hue.APIResponse[hue.Group]{Data: []hue.Group{
				{
					ID:       "zone-1",
					Metadata: hue.NamedMetadata{Name: "Upstairs"},
					Services: []hue.ResourceReference{{RID: "grouped-2", RType: hue.ResourceTypeGroupedLight}},
				},
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/clip/v2/resource/zone/zone-1":
			writeJSONFixture(t, w, hue.APIResponse[hue.Group]{Data: []hue.Group{
				{
					ID:       "zone-1",
					Services: []hue.ResourceReference{{RID: "grouped-2", RType: hue.ResourceTypeGroupedLight}},
				},
			}})
		case r.Method == http.MethodPut:
			putPath = r.URL.Path
			decodeJSONFixture(t, r, &gotBody)
			writeJSONFixture(t, w, hue.APIResponse[hue.ResourceIdentifier]{
				Data: []hue.ResourceIdentifier{{RID: "grouped-2", RType: hue.ResourceTypeGroupedLight}},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	withClient(t, client)

	out, err := execute(t, "zone", "off", "Upstairs")
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if !strings.Contains(out, `Updated "Upstairs" (1 resource(s)).`) {
		t.Fatalf("output = %q, want update confirmation", out)
	}
	if putPath != "/clip/v2/resource/grouped_light/grouped-2" {
		t.Fatalf("PUT path = %q, want grouped_light/grouped-2", putPath)
	}
	if gotBody.On == nil || gotBody.On.On {
		t.Fatalf("request body On = %+v, want on=false", gotBody.On)
	}
}
