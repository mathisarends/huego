package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

const (
	agentRoomID   = "10000000-0000-0000-0000-000000000001"
	agentSceneID  = "20000000-0000-0000-0000-000000000001"
	agentDeviceID = "30000000-0000-0000-0000-000000000001"
	agentOtherID  = "40000000-0000-0000-0000-000000000001"
)

func resourceFixture(id, kind, name string) hue.Resource {
	return hue.Resource{"id": jsonValue(id), "type": jsonValue(kind), "metadata": jsonValue(map[string]string{"name": name})}
}

func fakeInventory(t *testing.T, resources map[string]hue.Resource, onWrite func(string, hue.Resource)) *hue.Client {
	t.Helper()
	return newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/clip/v2/resource/")
		if r.Method == http.MethodGet {
			data := []hue.Resource{}
			if r.URL.Path == "/clip/v2/resource" {
				for _, resource := range resources {
					data = append(data, resource)
				}
			} else if strings.Contains(path, "/") {
				if resource, ok := resources[path]; ok {
					data = append(data, resource)
				}
			} else {
				for key, resource := range resources {
					if strings.HasPrefix(key, path+"/") {
						data = append(data, resource)
					}
				}
			}
			writeJSONFixture(t, w, hue.ResourceResponse{Data: data})
			return
		}
		var payload hue.Resource
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if onWrite != nil {
			onWrite(path, payload)
		}
		if r.Method == http.MethodPost {
			payload["id"] = jsonValue(agentSceneID)
			payload["type"] = jsonValue(path)
			resources[path+"/"+agentSceneID] = payload
		} else {
			existing := resources[path]
			for key, value := range payload {
				existing[key] = value
			}
		}
		writeJSONFixture(t, w, map[string]any{"errors": []any{}, "data": []map[string]string{{"rid": agentSceneID, "rtype": strings.Split(path, "/")[0]}}})
	})
}

func agentInventory() map[string]hue.Resource {
	room := resourceFixture(agentRoomID, "room", "Office")
	room["children"] = jsonValue([]hue.ResourceReference{{RID: agentDeviceID, RType: "device"}, {RID: agentOtherID, RType: "device"}})
	light := resourceFixture(testLightUUID, "light", "Desk")
	light["owner"] = jsonValue(hue.ResourceReference{RID: agentDeviceID, RType: "device"})
	light["on"] = json.RawMessage(`{"on":true}`)
	light["dimming"] = json.RawMessage(`{"brightness":75,"min_dim_level":0.2}`)
	light["color_temperature"] = json.RawMessage(`{"mirek":370,"mirek_valid":true,"mirek_schema":{"mirek_minimum":153,"mirek_maximum":500}}`)
	light["color"] = json.RawMessage(`{"xy":{"x":0.5,"y":0.4},"gamut_type":"C"}`)
	scene := resourceFixture(agentSceneID, "scene", "Relax")
	scene["group"] = jsonValue(hue.ResourceReference{RID: agentRoomID, RType: "room"})
	scene["actions"] = jsonValue([]hue.Resource{{"target": jsonValue(hue.ResourceReference{RID: testLightUUID, RType: "light"}), "action": json.RawMessage(`{"on":{"on":false},"dimming":{"brightness":20},"future":null}`)}, {"target": jsonValue(hue.ResourceReference{RID: agentOtherID, RType: "light"}), "action": json.RawMessage(`{"on":{"on":true}}`)}})
	return map[string]hue.Resource{"room/" + agentRoomID: room, "light/" + testLightUUID: light, "scene/" + agentSceneID: scene}
}

func TestAgentSceneScopeAndActionPreservation(t *testing.T) {
	resources := agentInventory()
	duplicate := resourceFixture(agentOtherID, "scene", "Relax")
	duplicate["group"] = jsonValue(hue.ResourceReference{RID: agentOtherID, RType: "room"})
	resources["scene/"+agentOtherID] = duplicate
	writes := 0
	client := fakeInventory(t, resources, func(path string, payload hue.Resource) {
		writes++
		if path != "scene/"+agentSceneID {
			t.Error(path)
		}
		actions := decodeField[[]hue.Resource](payload, "actions")
		if len(actions) != 2 {
			t.Fatalf("lost other action: %s", payload["actions"])
		}
		action := decodeField[hue.Resource](actions[0], "action")
		if !sameJSON(action["on"], json.RawMessage(`{"on":false}`)) || string(action["future"]) != "null" {
			t.Fatalf("lost existing settings: %v", action)
		}
	})
	withClient(t, client)
	if _, err := execute(t, "scene", "set-light", "Relax", "Desk", "--room", "Office", "--brightness", "42", "--json"); err != nil {
		t.Fatal(err)
	}
	if writes != 1 {
		t.Fatal(writes)
	}
	out, err := execute(t, "scene", "list", "--room", "Office", "--json")
	if err != nil || !strings.Contains(out, agentRoomID) || strings.Contains(out, `"id":"`+agentOtherID+`"`) {
		t.Fatal(out, err)
	}
	if _, err := execute(t, "scene", "delete", "Relax", "--dry-run"); err == nil {
		t.Fatal("unscoped duplicate accepted")
	}
}

func TestAgentMembershipIdempotentAndPreservesOthers(t *testing.T) {
	resources := agentInventory()
	writes := 0
	withClient(t, fakeInventory(t, resources, func(_ string, _ hue.Resource) { writes++ }))
	if _, err := execute(t, "room", "add", "Office", "Desk", "--json"); err != nil {
		t.Fatal(err)
	}
	if writes != 0 {
		t.Fatal("adding existing member wrote")
	}
	for range 2 {
		if _, err := execute(t, "room", "remove", "Office", "Desk", "--json"); err != nil {
			t.Fatal(err)
		}
	}
	children := decodeField[[]hue.ResourceReference](resources["room/"+agentRoomID], "children")
	if writes != 1 || len(children) != 1 || children[0].RID != agentOtherID {
		t.Fatalf("writes=%d children=%v", writes, children)
	}
}

func TestAgentSnapshotAndEnsureReuseID(t *testing.T) {
	resources := agentInventory()
	delete(resources, "scene/"+agentSceneID)
	writes := 0
	withClient(t, fakeInventory(t, resources, func(_ string, payload hue.Resource) {
		writes++
		actions := decodeField[[]hue.Resource](payload, "actions")
		if len(actions) != 1 {
			t.Fatalf("snapshot wrong group: %s", payload["actions"])
		}
		action := decodeField[hue.Resource](actions[0], "action")
		if len(action["color"]) != 0 || strings.Contains(string(action["color_temperature"]), "mirek_valid") || strings.Contains(string(action["dimming"]), "min_dim_level") {
			t.Fatal("snapshot contains competing color or read-only fields", action)
		}
	}))
	for range 2 {
		if _, err := execute(t, "scene", "snapshot", "Working", "--room", "Office", "--ensure", "--json"); err != nil {
			t.Fatal(err)
		}
	}
	if writes != 1 {
		t.Fatalf("repeated snapshot created duplicates: %d", writes)
	}
}

func TestAgentSceneCloneExcludesReadOnlyFields(t *testing.T) {
	resources := agentInventory()
	resources["scene/"+agentSceneID]["status"] = json.RawMessage(`{"active":"inactive"}`)
	writes := 0
	withClient(t, fakeInventory(t, resources, func(_ string, payload hue.Resource) {
		writes++
		for _, key := range []string{"id", "type", "status"} {
			if _, ok := payload[key]; ok {
				t.Errorf("copied read-only %s", key)
			}
		}
		if decodeField[hue.NamedMetadata](payload, "metadata").Name != "Copy" {
			t.Fatal(payload)
		}
	}))
	if _, err := execute(t, "scene", "clone", agentSceneID, "Copy", "--json"); err != nil {
		t.Fatal(err)
	}
	if writes != 1 {
		t.Fatal(writes)
	}
}

func TestAgentConcurrentMembershipChangeStopsWrite(t *testing.T) {
	reads := 0
	withClient(t, newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Error("concurrent change was overwritten")
		}
		if strings.Contains(r.URL.Path, "/light/") {
			writeJSONFixture(t, w, hue.ResourceResponse{Data: []hue.Resource{agentInventory()["light/"+testLightUUID]}})
			return
		}
		reads++
		room := agentInventory()["room/"+agentRoomID]
		if reads > 1 {
			room["children"] = json.RawMessage(`[]`)
		}
		writeJSONFixture(t, w, hue.ResourceResponse{Data: []hue.Resource{room}})
	}))
	if _, err := execute(t, "room", "remove", agentRoomID, testLightUUID); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatal(err)
	}
}

func TestAgentDryRunsValidateWithoutCredentials(t *testing.T) {
	withoutCredentials(t)
	for _, args := range [][]string{{"light", "on", testLightUUID, "--brightness", "20"}, {"light", "off", testLightUUID}, {"light", "color", testLightUUID, "red"}, {"light", "temperature", testLightUUID, "2700"}, {"light", "identify", testLightUUID}, {"scene", "activate", agentSceneID}} {
		out, err := execute(t, append(args, "--dry-run", "--json")...)
		if err != nil || !strings.Contains(out, `"schema":"valid"`) {
			t.Fatal(args, out, err)
		}
	}
	for _, args := range [][]string{{"scene", "create", "--data", "{}", "--dry-run"}, {"resource", "validate", "light", "--method", "PUT", "--data", `{"dimming":{"brightness":101}}`}, {"scene", "activate", agentSceneID, "--transition", "NaN", "--dry-run"}} {
		if _, err := execute(t, args...); err == nil {
			t.Fatal("invalid request accepted", args)
		}
	}
	for kind, examples := range requestExamples {
		for method := range examples {
			if _, err := execute(t, "resource", "example", kind, "--method", method, "--json"); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestAgentScriptConfigurationValidation(t *testing.T) {
	resources := agentInventory()
	script := resourceFixture(agentOtherID, "behavior_script", "Timer")
	script["configuration_schema"] = json.RawMessage(`{"type":"object","required":["seconds"],"properties":{"seconds":{"type":"integer","minimum":1}}}`)
	resources["behavior_script/"+agentOtherID] = script
	writes := 0
	withClient(t, fakeInventory(t, resources, func(string, hue.Resource) { writes++ }))
	payload := fmt.Sprintf(`{"script_id":%q,"enabled":false,"configuration":{"seconds":0}}`, agentOtherID)
	if _, err := execute(t, "behavior-instance", "create", "--data", payload, "--check"); err == nil {
		t.Fatal("invalid script configuration accepted")
	}
	if writes != 0 {
		t.Fatal(writes)
	}
}

func TestAgentStructuredErrors(t *testing.T) {
	for _, test := range []struct {
		err  error
		code string
	}{{&hue.Problem{Code: "ambiguous_target", Message: "ambiguous", Details: map[string]any{"candidate_ids": []string{"a", "b"}}}, "ambiguous_target"}, {&hue.APIError{StatusCode: 429, Message: "busy"}, "bridge_http_error"}, {context.DeadlineExceeded, "timeout"}} {
		var out bytes.Buffer
		writeCommandError(&out, test.err)
		var result map[string]hue.Problem
		if err := json.Unmarshal(out.Bytes(), &result); err != nil || result["error"].Code != test.code {
			t.Fatal(out.String(), err)
		}
	}
}

func TestAgentWatchFilteringAndWait(t *testing.T) {
	withClient(t, newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/eventstream/clip/v2" {
			_, _ = fmt.Fprintf(w, "id: one\ndata: [{\"type\":\"update\",\"data\":[{\"id\":%q,\"type\":\"light\",\"on\":{\"on\":true}},{\"id\":\"sensor\",\"type\":\"motion\"}]}]\n\n", testLightUUID)
			return
		}
		_, _ = fmt.Fprintf(w, `{"errors":[],"data":[{"id":%q,"type":"light","on":{"on":true}}]}`, testLightUUID)
	}))
	out, err := execute(t, "watch", "--type", "light", "--count", "1", "--timeout", "1s", "--json")
	if err != nil || strings.Contains(out, "sensor") || !strings.Contains(out, "one") {
		t.Fatal(out, err)
	}
	out, err = execute(t, "resource", "wait", "light", testLightUUID, "--path", "/on/on", "--equals", "true", "--timeout", "1s", "--json")
	if err != nil || !strings.Contains(out, `"on":true`) {
		t.Fatal(out, err)
	}
	_, err = execute(t, "resource", "wait", "light", testLightUUID, "--path", "/on/on", "--equals", "false", "--interval", "100ms", "--timeout", "150ms")
	if err == nil {
		t.Fatal("unmet wait succeeded")
	}
}

func TestAgentSetupRejectsClosedInput(t *testing.T) {
	command := &cobra.Command{}
	command.SetIn(strings.NewReader(""))
	command.SetOut(io.Discard)
	start := time.Now()
	_, err := selectBridge(command, []hue.DiscoveredBridge{{ID: "a"}, {ID: "b"}})
	if err == nil || time.Since(start) > time.Second {
		t.Fatal("setup did not fail promptly", err)
	}
}

func TestAgentMoveFailureReportsProgressAndCanResume(t *testing.T) {
	resources := agentInventory()
	destination := resourceFixture(agentOtherID, "room", "Study")
	destination["children"] = json.RawMessage(`[]`)
	resources["room/"+agentOtherID] = destination
	fail := true
	sourceWrites, destWrites := 0, 0
	withClient(t, newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/clip/v2/resource/")
		if r.Method == "GET" {
			data := []hue.Resource{}
			if path == "room" {
				data = append(data, resources["room/"+agentRoomID], destination)
			} else {
				data = append(data, resources[path])
			}
			writeJSONFixture(t, w, hue.ResourceResponse{Data: data})
			return
		}
		if path == "room/"+agentOtherID {
			destWrites++
			if fail {
				fail = false
				w.WriteHeader(503)
				_, _ = io.WriteString(w, `{"errors":[{"description":"busy"}],"data":[]}`)
				return
			}
		} else {
			sourceWrites++
		}
		var payload hue.Resource
		_ = json.NewDecoder(r.Body).Decode(&payload)
		resources[path]["children"] = payload["children"]
		writeJSONFixture(t, w, hue.ResourceResponse{Data: []hue.Resource{{"rid": jsonValue(strings.TrimPrefix(path, "room/")), "rtype": jsonValue("room")}}})
	}))
	_, err := execute(t, "room", "move", testLightUUID, "Study", "--json")
	problem := commandProblem(err)
	if err == nil || problem.Code != "workflow_failed" {
		t.Fatal(err)
	}
	details := problem.Details.(map[string]any)
	if len(details["completed"].([]hue.ResourceResponse)) != 1 || details["failed_step"] != 1 {
		t.Fatal(details)
	}
	if _, err := execute(t, "room", "move", testLightUUID, "Study", "--json"); err != nil {
		t.Fatal(err)
	}
	if sourceWrites != 1 || destWrites != 2 {
		t.Fatal(sourceWrites, destWrites)
	}
	if len(decodeField[[]hue.ResourceReference](destination, "children")) != 1 {
		t.Fatal(destination)
	}
}

func TestAgentScopedCreationAndEntertainmentPreview(t *testing.T) {
	resources := agentInventory()
	withClient(t, fakeInventory(t, resources, func(string, hue.Resource) { t.Error("dry run wrote") }))
	out, err := execute(t, "scene", "create", "--room", "Office", "--data", `{"metadata":{"name":"New"},"actions":[]}`, "--dry-run", "--json")
	if err != nil || !strings.Contains(out, agentRoomID) {
		t.Fatal(out, err)
	}
	if _, err := execute(t, "scene", "create", "--room", "Office", "--data", `{"metadata":{"name":"New"},"group":{"rid":"40000000-0000-0000-0000-000000000001","rtype":"room"},"actions":[]}`, "--dry-run"); err == nil {
		t.Fatal("mismatched scope accepted")
	}
	withoutCredentials(t)
	out, err = execute(t, "entertainment-configuration", "stream", agentOtherID, "--data", `{"frames":[{"0":[1,0,0]}]}`, "--dry-run", "--json")
	if err != nil || !strings.Contains(out, `"writes":false`) {
		t.Fatal(out, err)
	}
}

func TestJSONComparisonPreservesNumbers(t *testing.T) {
	for _, test := range []struct {
		left, right string
		equal       bool
	}{
		{`{"n":9007199254740992}`, `{"n":9007199254740993}`, false},
		{`[1,1e20,0.25]`, `[1.0,100000000000000000000,25e-2]`, true},
		{`{"a":true,"b":null}`, `{"b":null,"a":true}`, true},
		{`{"a":true}`, `{"a":1}`, false},
	} {
		if got := sameJSON(json.RawMessage(test.left), json.RawMessage(test.right)); got != test.equal {
			t.Errorf("sameJSON(%s, %s) = %v", test.left, test.right, got)
		}
	}
}

func TestJSONRequestedBeforeArgumentParsing(t *testing.T) {
	for _, test := range []struct {
		args []string
		want bool
	}{{[]string{"invalid-command", "--json"}, true}, {[]string{"--json=true"}, true}, {[]string{"--json", "--json=false"}, false}, {[]string{"--", "--json"}, false}} {
		if got := jsonRequested(test.args); got != test.want {
			t.Fatal(test.args, got)
		}
	}
}
