package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/mathisarends/huego/internal/hue"
)

func TestNativeGroupsGetAndWriteContract(t *testing.T) {
	for _, capability := range hue.Catalog().Resources {
		t.Run(capability.ResourceType, func(t *testing.T) {
			name := strings.ReplaceAll(capability.ResourceType, "_", "-")
			for _, operation := range []struct {
				command string
				method  string
				item    bool
			}{
				{"get", "GET", true}, {"create", "POST", false}, {"update", "PUT", true}, {"delete", "DELETE", true},
			} {
				calls := 0
				client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
					calls++
					path := "/clip/v2/resource/" + capability.ResourceType
					if operation.item {
						path += "/" + testLightUUID
					}
					if r.Method != operation.method || r.URL.Path != path {
						t.Errorf("request = %s %s, want %s %s", r.Method, r.URL.Path, operation.method, path)
					}
					_, _ = fmt.Fprintf(w, `{"errors":[],"data":[{"id":%q,"future":null}]}`, testLightUUID)
				})
				withClient(t, client)
				args := []string{name, operation.command}
				if operation.item {
					args = append(args, testLightUUID)
				}
				if operation.method == "POST" || operation.method == "PUT" {
					args = append(args, "--data", `{}`)
				}
				out, err := execute(t, append(args, "--json")...)
				if !capability.Supports(operation.method, operation.item) {
					if err == nil || calls != 0 {
						t.Errorf("unsupported %v: calls=%d err=%v", args, calls, err)
					}
					continue
				}
				if err != nil || calls != 1 || !json.Valid([]byte(out)) || !strings.Contains(out, `"future":null`) {
					t.Fatalf("%v: calls=%d out=%q err=%v", args, calls, out, err)
				}
			}
		})
	}
}

func TestNativeListAndUnderscoreAliases(t *testing.T) {
	for _, kind := range []string{"device", "motion", "device_power", "smart_scene", "behavior_instance", "entertainment_configuration"} {
		t.Run(kind, func(t *testing.T) {
			client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Path != "/clip/v2/resource/"+kind {
					t.Errorf("request=%s %s", r.Method, r.URL.Path)
				}
				_, _ = io.WriteString(w, `{"errors":[],"data":[]}`)
			})
			withClient(t, client)
			out, err := execute(t, kind, "list", "--json")
			if err != nil || out != "{\"errors\":[],\"data\":[]}\n" {
				t.Fatalf("out=%q err=%v", out, err)
			}
		})
	}
}

func TestTaskCommandPayloadsAndDryRuns(t *testing.T) {
	for _, test := range []struct {
		args []string
		kind string
		body string
	}{
		{[]string{"device", "rename", testLightUUID, "Desk"}, "device", `{"metadata":{"name":"Desk"}}`},
		{[]string{"room", "rename", testLightUUID, "Office"}, "room", `{"metadata":{"name":"Office"}}`},
		{[]string{"scene", "rename", testLightUUID, "Relax"}, "scene", `{"metadata":{"name":"Relax"}}`},
		{[]string{"device", "identify", testLightUUID}, "device", `{"identify":{"action":"identify"}}`},
		{[]string{"device", "usertest", testLightUUID, "false"}, "device", `{"usertest":{"usertest":false}}`},
		{[]string{"motion", "sensitivity", testLightUUID, "0"}, "motion", `{"sensitivity":{"sensitivity":0}}`},
		{[]string{"motion", "enable", testLightUUID}, "motion", `{"enabled":true}`},
		{[]string{"contact", "disable", testLightUUID}, "contact", `{"enabled":false}`},
		{[]string{"behavior-instance", "enable", testLightUUID}, "behavior_instance", `{"enabled":true}`},
		{[]string{"smart-scene", "activate", testLightUUID}, "smart_scene", `{"recall":{"action":"activate"}}`},
		{[]string{"smart-scene", "deactivate", testLightUUID}, "smart_scene", `{"recall":{"action":"deactivate"}}`},
		{[]string{"entertainment-configuration", "start", testLightUUID}, "entertainment_configuration", `{"action":"start"}`},
		{[]string{"entertainment-configuration", "stop", testLightUUID}, "entertainment_configuration", `{"action":"stop"}`},
		{[]string{"zigbee-device-discovery", "search", testLightUUID}, "zigbee_device_discovery", `{"action":{"action_type":"search"}}`},
		{[]string{"device-software-update", "install", testLightUUID}, "device_software_update", `{"install":{"install_state":"install"}}`},
		{[]string{"geofence-client", "at-home", testLightUUID, "true"}, "geofence_client", `{"is_at_home":true}`},
		{[]string{"geofence-client", "rename", testLightUUID, "Phone"}, "geofence_client", `{"name":"Phone"}`},
		{[]string{"light", "effect", testLightUUID, "candle"}, "light", `{"effects_v2":{"action":{"effect":"candle"}}}`},
		{[]string{"light", "effect", testLightUUID, "no_effect"}, "light", `{"effects_v2":{"action":{"effect":"no_effect"}}}`},
		{[]string{"light", "powerup", testLightUUID, "last_on_state"}, "light", `{"powerup":{"preset":"last_on_state"}}`},
	} {
		t.Run(strings.Join(test.args[:2], " "), func(t *testing.T) {
			calls := 0
			client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				body, _ := io.ReadAll(r.Body)
				if r.Method != "PUT" || r.URL.Path != "/clip/v2/resource/"+test.kind+"/"+testLightUUID {
					t.Errorf("request=%s %s", r.Method, r.URL.Path)
				}
				assertSameJSON(t, body, []byte(test.body))
				_, _ = io.WriteString(w, `{"errors":[],"data":[]}`)
			})
			withClient(t, client)
			if _, err := execute(t, append(test.args, "--json")...); err != nil || calls != 1 {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
			withoutCredentials(t)
			out, err := execute(t, append(test.args, "--dry-run", "--json")...)
			if err != nil {
				t.Fatal(err)
			}
			var plan requestPlan
			if err := json.Unmarshal([]byte(out), &plan); err != nil {
				t.Fatal(err)
			}
			assertSameJSON(t, plan.Body, []byte(test.body))
		})
	}
}

func assertSameJSON(t *testing.T, got, want []byte) {
	t.Helper()
	var actual, expected any
	if err := json.Unmarshal(got, &actual); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(want, &expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("body=%s, want %s", got, want)
	}
}

func TestTaskValidationBeforeCredentials(t *testing.T) {
	withoutCredentials(t)
	for _, args := range [][]string{
		{"device", "rename", testLightUUID, " "},
		{"device", "rename", testLightUUID, strings.Repeat("x", 33)},
		{"motion", "sensitivity", testLightUUID, "--", "-1"},
		{"motion", "sensitivity", testLightUUID, "a"},
		{"device", "usertest", testLightUUID, "yes"},
		{"light", "powerup", testLightUUID, "custom"},
		{"light", "effect", testLightUUID, " "},
	} {
		if _, err := execute(t, args...); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
}
