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

func TestLightsList(t *testing.T) {
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

	lights, err := client.Lights.List(t.Context())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(lights) != 1 || lights[0].Name() != "Desk Lamp" {
		t.Fatalf("List() = %+v, want one light named Desk Lamp", lights)
	}
}

func TestLightsFindByNameFuzzy(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSONFixture(t, w, APIResponse[Light]{Data: []Light{
			{ID: "1", Metadata: NamedMetadata{Name: "Living Room Lamp"}},
			{ID: "2", Metadata: NamedMetadata{Name: "Kitchen Light"}},
		}})
	})

	light, err := client.Lights.FindByName(t.Context(), "living room lamp")
	if err != nil {
		t.Fatalf("FindByName() error = %v", err)
	}
	if light.ID != "1" {
		t.Fatalf("FindByName() = %+v, want ID 1", light)
	}
}

func TestLightsFindByNameNotFound(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSONFixture(t, w, APIResponse[Light]{Data: []Light{
			{ID: "1", Metadata: NamedMetadata{Name: "Desk Lamp"}},
		}})
	})

	if _, err := client.Lights.FindByName(t.Context(), "totally unrelated"); err == nil {
		t.Fatal("FindByName() error = nil, want error")
	}
}

func TestLightsApplySendsPut(t *testing.T) {
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
	_, err := TurnOn(t.Context(), client.Lights, "abc", nil, nil, nil)
	if err != nil {
		t.Fatalf("TurnOn() error = %v", err)
	}
	if gotBody.On == nil || gotBody.On.On != on {
		t.Fatalf("request body On = %+v, want on=true", gotBody.On)
	}
}

func TestGroupsAppliesViaGroupedLight(t *testing.T) {
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

	if _, err := TurnOff(t.Context(), client.Rooms, "room-1", nil); err != nil {
		t.Fatalf("TurnOff() error = %v", err)
	}
	if putPath != "/clip/v2/resource/grouped_light/grouped-1" {
		t.Fatalf("PUT path = %q, want grouped_light/grouped-1", putPath)
	}
}

func TestScenesActivate(t *testing.T) {
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

func TestLightsIsOn(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSONFixture(t, w, APIResponse[Light]{Data: []Light{
			{ID: "1", On: OnState{On: true}},
		}})
	})

	on, err := client.IsOn(t.Context(), "1")
	if err != nil {
		t.Fatalf("IsOn() error = %v", err)
	}
	if !on {
		t.Fatal("IsOn() = false, want true")
	}
}

func TestGroupsIsOn(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/clip/v2/resource/room/room-1":
			writeJSONFixture(t, w, APIResponse[Group]{Data: []Group{{
				ID:       "room-1",
				Services: []ResourceReference{{RID: "grouped-1", RType: ResourceTypeGroupedLight}},
			}}})
		case "/clip/v2/resource/grouped_light/grouped-1":
			writeJSONFixture(t, w, APIResponse[GroupedLight]{Data: []GroupedLight{
				{ID: "grouped-1", On: &OnState{On: true}},
			}})
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}
	})

	on, err := client.Rooms.IsOn(t.Context(), "room-1")
	if err != nil {
		t.Fatalf("IsOn() error = %v", err)
	}
	if !on {
		t.Fatal("IsOn() = false, want true")
	}
}

func TestGroupsIsOnFalseWhenOnStateMissing(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/clip/v2/resource/room/room-1":
			writeJSONFixture(t, w, APIResponse[Group]{Data: []Group{{
				ID:       "room-1",
				Services: []ResourceReference{{RID: "grouped-1", RType: ResourceTypeGroupedLight}},
			}}})
		case "/clip/v2/resource/grouped_light/grouped-1":
			writeJSONFixture(t, w, APIResponse[GroupedLight]{Data: []GroupedLight{
				{ID: "grouped-1"},
			}})
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}
	})

	on, err := client.Rooms.IsOn(t.Context(), "room-1")
	if err != nil {
		t.Fatalf("IsOn() error = %v", err)
	}
	if on {
		t.Fatal("IsOn() = true, want false")
	}
}

func TestGroupsGroupedLightNoServiceMatch(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSONFixture(t, w, APIResponse[Group]{Data: []Group{{
			ID:       "room-1",
			Services: []ResourceReference{{RID: "some-sensor", RType: "device_power"}},
		}}})
	})

	if _, err := client.Rooms.IsOn(t.Context(), "room-1"); err == nil {
		t.Fatal("IsOn() error = nil, want error for a room with no grouped_light service")
	}
}

func TestResolveTargetPassesThroughUUID(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("ResolveTarget should not call the bridge for a UUID target")
	})

	uuid := "01234567-89ab-cdef-0123-456789abcdef"
	id, err := client.Lights.ResolveTarget(t.Context(), uuid)
	if err != nil {
		t.Fatalf("ResolveTarget() error = %v", err)
	}
	if id != uuid {
		t.Fatalf("ResolveTarget() = %q, want %q", id, uuid)
	}
}

func TestResolveTargetLooksUpByName(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSONFixture(t, w, APIResponse[Light]{Data: []Light{
			{ID: "abc", Metadata: NamedMetadata{Name: "Desk Lamp"}},
		}})
	})

	id, err := client.Lights.ResolveTarget(t.Context(), "Desk Lamp")
	if err != nil {
		t.Fatalf("ResolveTarget() error = %v", err)
	}
	if id != "abc" {
		t.Fatalf("ResolveTarget() = %q, want abc", id)
	}
}

func TestGetOneNotFound(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSONFixture(t, w, APIResponse[Light]{Data: []Light{}})
	})

	if _, err := client.Lights.GetOne(t.Context(), "missing"); err == nil {
		t.Fatal("GetOne() error = nil, want error")
	}
}

func TestFindByNameNoCloseMatch(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSONFixture(t, w, APIResponse[Light]{Data: []Light{
			{ID: "1", Metadata: NamedMetadata{Name: "Desk Lamp"}},
			{ID: "2", Metadata: NamedMetadata{Name: "Kitchen Light"}},
		}})
	})

	_, err := client.Lights.FindByName(t.Context(), "totally unrelated")
	if err == nil {
		t.Fatal("FindByName() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "Desk Lamp") || !strings.Contains(err.Error(), "Kitchen Light") {
		t.Fatalf("error = %q, want it to list known names", err.Error())
	}
}

func TestFindByNameNoResources(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSONFixture(t, w, APIResponse[Light]{Data: []Light{}})
	})

	_, err := client.Lights.FindByName(t.Context(), "anything")
	if err == nil {
		t.Fatal("FindByName() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "none") {
		t.Fatalf("error = %q, want it to say known names: none", err.Error())
	}
}

func TestScenesActivateDynamicWithTransition(t *testing.T) {
	var gotRequest SceneRecallRequest
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotRequest)
		writeJSONFixture(t, w, APIResponse[ResourceIdentifier]{
			Data: []ResourceIdentifier{{RID: "scene-1", RType: ResourceTypeScene}},
		})
	})

	transition := 2.0
	_, err := client.Activate(t.Context(), "scene-1", SceneActivation{Dynamic: true, Transition: &transition})
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	if gotRequest.Recall.Action != SceneActionDynamicPalette {
		t.Fatalf("Recall.Action = %v, want dynamic_palette", gotRequest.Recall.Action)
	}
	if gotRequest.Recall.Duration == nil || *gotRequest.Recall.Duration != 2000 {
		t.Fatalf("Recall.Duration = %v, want 2000", gotRequest.Recall.Duration)
	}
}

func TestAPIErrorOnNon2xx(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errors":[{"description":"unauthorized user"}]}`))
	})

	_, err := client.Lights.List(t.Context())
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
