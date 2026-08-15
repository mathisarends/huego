package hue

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mathisarends/huego/internal/config"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)

	ip := strings.TrimPrefix(srv.URL, "https://")
	return NewClient(config.Bridge{IP: ip, AppKey: "test-app-key"})
}

func writeJSONFixture(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

func TestLightNamespaceList(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("hue-application-key"); got != "test-app-key" {
			t.Errorf("hue-application-key header = %q, want test-app-key", got)
		}
		if r.URL.Path != "/clip/v2/resource/light" {
			t.Errorf("path = %q, want /clip/v2/resource/light", r.URL.Path)
		}
		writeJSONFixture(t, w, APIResponse[Light]{Data: []Light{
			{ID: "1", Metadata: NamedMetadata{Name: "Desk Lamp"}, On: OnState{On: true}},
		}})
	})

	lights, err := client.LightNamespace.List(t.Context())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(lights) != 1 || lights[0].Name() != "Desk Lamp" {
		t.Fatalf("List() = %+v, want one light named Desk Lamp", lights)
	}
}

func TestLightNamespaceFindByNameFuzzy(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSONFixture(t, w, APIResponse[Light]{Data: []Light{
			{ID: "1", Metadata: NamedMetadata{Name: "Living Room Lamp"}},
			{ID: "2", Metadata: NamedMetadata{Name: "Kitchen Light"}},
		}})
	})

	light, err := client.LightNamespace.FindByName(t.Context(), "living room lamp")
	if err != nil {
		t.Fatalf("FindByName() error = %v", err)
	}
	if light.ID != "1" {
		t.Fatalf("FindByName() = %+v, want ID 1", light)
	}
}

func TestLightNamespaceFindByNameNotFound(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSONFixture(t, w, APIResponse[Light]{Data: []Light{
			{ID: "1", Metadata: NamedMetadata{Name: "Desk Lamp"}},
		}})
	})

	if _, err := client.LightNamespace.FindByName(t.Context(), "totally unrelated"); err == nil {
		t.Fatal("FindByName() error = nil, want error")
	}
}

func TestLightNamespaceApplySendsPut(t *testing.T) {
	var gotBody LightUpdate
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/clip/v2/resource/light/abc" {
			t.Errorf("path = %q, want /clip/v2/resource/light/abc", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		writeJSONFixture(t, w, APIResponse[ResourceIdentifier]{
			Data: []ResourceIdentifier{{RID: "abc", RType: ResourceTypeLight}},
		})
	})

	on := true
	_, err := TurnOn(t.Context(), client.LightNamespace, "abc", nil, nil, nil)
	if err != nil {
		t.Fatalf("TurnOn() error = %v", err)
	}
	if gotBody.On == nil || gotBody.On.On != on {
		t.Fatalf("request body On = %+v, want on=true", gotBody.On)
	}
}

func TestGroupNamespaceAppliesViaGroupedLight(t *testing.T) {
	var putPath string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/clip/v2/resource/room/room-1":
			writeJSONFixture(t, w, APIResponse[Group]{Data: []Group{{
				ID:       "room-1",
				Metadata: NamedMetadata{Name: "Kitchen"},
				Services: []ResourceReference{{RID: "grouped-1", RType: ResourceTypeGroupedLight}},
			}}})
		case r.Method == http.MethodPut:
			putPath = r.URL.Path
			writeJSONFixture(t, w, APIResponse[ResourceIdentifier]{
				Data: []ResourceIdentifier{{RID: "grouped-1", RType: ResourceTypeGroupedLight}},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	if _, err := TurnOff(t.Context(), client.RoomNamespace, "room-1", nil); err != nil {
		t.Fatalf("TurnOff() error = %v", err)
	}
	if putPath != "/clip/v2/resource/grouped_light/grouped-1" {
		t.Fatalf("PUT path = %q, want grouped_light/grouped-1", putPath)
	}
}

func TestSceneNamespaceActivate(t *testing.T) {
	var gotRequest SceneRecallRequest
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotRequest)
		writeJSONFixture(t, w, APIResponse[ResourceIdentifier]{
			Data: []ResourceIdentifier{{RID: "scene-1", RType: ResourceTypeScene}},
		})
	})

	brightness := 80.0
	_, err := client.Activate(t.Context(), "scene-1", SceneActivation{Brightness: &brightness})
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	if gotRequest.Recall.Action != SceneActionActive {
		t.Fatalf("Recall.Action = %v, want active", gotRequest.Recall.Action)
	}
	if gotRequest.Recall.Dimming == nil || gotRequest.Recall.Dimming.Brightness != 80 {
		t.Fatalf("Recall.Dimming = %+v, want brightness 80", gotRequest.Recall.Dimming)
	}
}

func TestAPIErrorOnNon2xx(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errors":[{"description":"unauthorized user"}]}`))
	})

	_, err := client.LightNamespace.List(t.Context())
	if err == nil {
		t.Fatal("List() error = nil, want error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("StatusCode = %d, want 401", apiErr.StatusCode)
	}
}
