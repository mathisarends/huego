package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const (
	officeRoomID   = "aaaaaaaa-0000-0000-0000-000000000001"
	kitchenRoomID  = "aaaaaaaa-0000-0000-0000-000000000002"
	officeSmartID  = "bbbbbbbb-0000-0000-0000-000000000001"
	kitchenSmartID = "bbbbbbbb-0000-0000-0000-000000000002"
	wakeUpID       = "cccccccc-0000-0000-0000-000000000001"
	wakeUpScriptID = "dddddddd-0000-0000-0000-000000000001"
)

// newRoutedBridge serves fixed GET bodies by path below /clip/v2/resource and
// records PUT requests as "<path> <body>".
func newRoutedBridge(t *testing.T, gets map[string]string, puts *[]string) {
	t.Helper()
	withClient(t, newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/clip/v2/resource")
		if r.Method == http.MethodPut {
			body, _ := io.ReadAll(r.Body)
			*puts = append(*puts, path+" "+string(body))
			id := path[strings.LastIndex(path, "/")+1:]
			_, _ = fmt.Fprintf(w, `{"errors":[],"data":[{"rid":%q,"rtype":"x"}]}`, id)
			return
		}
		body, ok := gets[path]
		if !ok {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
}

var smartSceneGets = map[string]string{
	"/smart_scene": `{"errors":[],"data":[
		{"id":"` + officeSmartID + `","metadata":{"name":"Natural light"},"group":{"rid":"` + officeRoomID + `","rtype":"room"},"state":"active"},
		{"id":"` + kitchenSmartID + `","metadata":{"name":"Natural light"},"group":{"rid":"` + kitchenRoomID + `","rtype":"room"},"state":"inactive"}]}`,
	"/room": `{"errors":[],"data":[
		{"id":"` + officeRoomID + `","metadata":{"name":"Office"},"children":[],"services":[]},
		{"id":"` + kitchenRoomID + `","metadata":{"name":"Kitchen"},"children":[],"services":[]}]}`,
	"/zone": `{"errors":[],"data":[]}`,
}

func TestSmartSceneList(t *testing.T) {
	newRoutedBridge(t, smartSceneGets, nil)

	out, err := execute(t, "smart-scene", "list", "--json")
	if err != nil {
		t.Fatalf("smart-scene list: %v", err)
	}
	var views []smartSceneView
	if err := json.Unmarshal([]byte(out), &views); err != nil {
		t.Fatalf("output %q is not JSON: %v", out, err)
	}
	if len(views) != 2 || views[0].Target != "Office/Natural light" || views[0].State != "active" {
		t.Fatalf("views = %+v", views)
	}

	out, err = execute(t, "smart-scene", "list", "--group", "kitchen", "--names")
	if err != nil || out != "Kitchen/Natural light\n" {
		t.Fatalf("smart-scene list --group = %q, %v", out, err)
	}

	_, err = execute(t, "smart-scene", "list", "--group", "Attic")
	if _, code := classifyError(err); code != exitNotFound || !strings.Contains(err.Error(), "Known: Kitchen, Office") {
		t.Fatalf("err = %v, want not found listing the known rooms", err)
	}
}

func TestSmartSceneGet(t *testing.T) {
	newRoutedBridge(t, smartSceneGets, nil)

	out, err := execute(t, "smart-scene", "get", "Kitchen/Natural light")
	if err != nil {
		t.Fatalf("smart-scene get: %v", err)
	}
	for _, want := range []string{"Natural light", kitchenSmartID, "Kitchen", "inactive"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestSmartSceneSharedNameIsAmbiguous(t *testing.T) {
	newRoutedBridge(t, smartSceneGets, nil)

	_, err := execute(t, "smart-scene", "activate", "natural light")
	report, code := classifyError(err)
	if code != exitNotFound || report.Code != "ambiguous" || len(report.Candidates) != 2 || report.Candidates[0] != "Office/Natural light" {
		t.Fatalf("report = %+v, want both qualified names as candidates", report)
	}
}

func TestSmartSceneActivateAndDeactivate(t *testing.T) {
	var puts []string
	newRoutedBridge(t, smartSceneGets, &puts)

	if _, err := execute(t, "smart-scene", "activate", "Office/Natural light"); err != nil {
		t.Fatalf("activate: %v", err)
	}
	out, err := execute(t, "smart-scene", "deactivate", kitchenSmartID)
	if err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if !strings.Contains(out, "Updated") {
		t.Fatalf("deactivate output = %q", out)
	}
	want := []string{
		"/smart_scene/" + officeSmartID + ` {"recall":{"action":"activate"}}`,
		"/smart_scene/" + kitchenSmartID + ` {"recall":{"action":"deactivate"}}`,
	}
	if strings.Join(puts, "\n") != strings.Join(want, "\n") {
		t.Fatalf("puts = %q, want %q", puts, want)
	}
}

var automationGets = map[string]string{
	"/behavior_instance": `{"errors":[],"data":[{"id":"` + wakeUpID + `","script_id":"` + wakeUpScriptID + `",
		"enabled":true,"status":"running","metadata":{"name":"Weekday wake-up"},"configuration":{"when":{"time_point":{"type":"time"}}}}]}`,
	"/behavior_script": `{"errors":[],"data":[{"id":"` + wakeUpScriptID + `","metadata":{"name":"Wake up","category":"automation"}}]}`,
}

func TestAutomationList(t *testing.T) {
	newRoutedBridge(t, automationGets, nil)

	out, err := execute(t, "automation", "list")
	if err != nil {
		t.Fatalf("automation list: %v", err)
	}
	for _, want := range []string{"Weekday wake-up", "Wake up", "yes", "running", wakeUpID} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestAutomationGetIncludesConfigurationInJSON(t *testing.T) {
	newRoutedBridge(t, automationGets, nil)

	out, err := execute(t, "automation", "get", "weekday wake-up", "--json")
	if err != nil {
		t.Fatalf("automation get: %v", err)
	}
	var view automationView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("output %q is not JSON: %v", out, err)
	}
	if view.Type != "Wake up" || !strings.Contains(string(view.Configuration), "time_point") {
		t.Fatalf("view = %+v", view)
	}
}

func TestAutomationEnableDisable(t *testing.T) {
	var puts []string
	newRoutedBridge(t, automationGets, &puts)

	if _, err := execute(t, "automation", "disable", "Weekday wake-up"); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if _, err := execute(t, "automation", "enable", wakeUpID); err != nil {
		t.Fatalf("enable: %v", err)
	}
	want := "/behavior_instance/" + wakeUpID + ` {"enabled":false}` + "\n" + "/behavior_instance/" + wakeUpID + ` {"enabled":true}`
	if strings.Join(puts, "\n") != want {
		t.Fatalf("puts = %q", puts)
	}
}

func TestEventsCommandNamesResources(t *testing.T) {
	gets := map[string]string{"": `{"errors":[],"data":[
		{"id":"` + officeRoomID + `","type":"room","metadata":{"name":"Office"}},
		{"id":"g1","type":"grouped_light","owner":{"rid":"` + officeRoomID + `","rtype":"room"}},
		{"id":"` + officeSmartID + `","type":"smart_scene","metadata":{"name":"Natural light"},"group":{"rid":"` + officeRoomID + `","rtype":"room"}},
		{"id":"h1","type":"bridge_home"}]}`}
	client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/eventstream/clip/v2" {
			_, _ = fmt.Fprint(w, `data: [{"creationtime":"2026-10-24T10:00:00Z","id":"e1","type":"update","data":[`+
				`{"id":"g1","type":"grouped_light","on":{"on":false}},`+
				`{"id":"`+officeSmartID+`","type":"smart_scene","state":"inactive"},`+
				`{"id":"h1","type":"bridge_home","dimming":{"brightness":40}}]}]`+"\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/clip/v2/resource")
		_, _ = w.Write([]byte(gets[path]))
	})
	withClient(t, client)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out := &syncBuffer{}
	done := make(chan error, 1)
	go func() { done <- executeContext(t, ctx, out, "events", "--type", "grouped_light,smart_scene") }()

	waitFor(t, func() bool { return strings.Contains(out.String(), "state=inactive") })
	cancel()
	if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("events: %v", err)
	}

	got := out.String()
	for _, want := range []string{"grouped_light  Office  off", "smart_scene  Office/Natural light  state=inactive"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "bridge_home") {
		t.Errorf("--type filter let bridge_home through:\n%s", got)
	}
}
