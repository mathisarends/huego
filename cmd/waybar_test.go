package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mathisarends/huego/internal/hue"
)

const (
	kitchenID        = "55555555-5555-5555-5555-555555555555"
	kitchenGroupedID = "66666666-6666-6666-6666-666666666666"
	kitchenDeviceID  = "77777777-7777-7777-7777-777777777777"
)

func waybarGets(homeOn bool) map[string]any {
	gets := homeGets()
	gets["grouped_light"] = hue.APIResponse[hue.GroupedLight]{Data: []hue.GroupedLight{
		{ID: homeGroupedID, On: &hue.OnState{On: homeOn}, Dimming: &hue.DimmingState{Brightness: 40}},
		{ID: kitchenGroupedID, On: &hue.OnState{On: homeOn}, Dimming: &hue.DimmingState{Brightness: 40}},
	}}
	gets["light"] = hue.APIResponse[hue.Light]{Data: []hue.Light{{
		ID: plainLightID, Metadata: hue.NamedMetadata{Name: "Desk"}, On: hue.OnState{On: homeOn},
		Owner: hue.ResourceReference{RID: kitchenDeviceID, RType: hue.ResourceTypeDevice},
	}}}
	gets["room"] = hue.APIResponse[hue.Group]{Data: []hue.Group{{
		ID: kitchenID, Type: hue.ResourceTypeRoom, Metadata: hue.NamedMetadata{Name: "Kitchen & Bar"},
		Children: []hue.ResourceReference{{RID: kitchenDeviceID, RType: hue.ResourceTypeDevice}},
		Services: []hue.ResourceReference{{RID: kitchenGroupedID, RType: hue.ResourceTypeGroupedLight}},
	}}}
	return gets
}

func decodeWaybarLines(t *testing.T, out string) []waybarStatus {
	t.Helper()
	var statuses []waybarStatus
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		var status waybarStatus
		if err := json.Unmarshal([]byte(line), &status); err != nil {
			t.Fatalf("line %q is not waybar JSON: %v", line, err)
		}
		statuses = append(statuses, status)
	}
	return statuses
}

func TestWaybarReportsHomeWithRoomsInTooltip(t *testing.T) {
	newRecordingBridge(t, waybarGets(true)).use(t)

	out, err := execute(t, "waybar")
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	statuses := decodeWaybarLines(t, out)
	if len(statuses) != 1 {
		t.Fatalf("got %d lines, want 1", len(statuses))
	}
	got := statuses[0]
	if got.Class != "on" || got.Alt != "on" || got.Text != "40%" || got.Percentage != 40 {
		t.Fatalf("status = %+v, want on at 40%%", got)
	}
	if !strings.Contains(got.Tooltip, "<b>Home: on, 40%</b>") || !strings.Contains(got.Tooltip, "Kitchen &amp; Bar: on, 40%") {
		t.Fatalf("tooltip = %q, want home headline and escaped room line", got.Tooltip)
	}
}

func TestWaybarRoomOff(t *testing.T) {
	newRecordingBridge(t, waybarGets(false)).use(t)

	out, err := execute(t, "waybar", "--room", "Kitchen & Bar")
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	got := decodeWaybarLines(t, out)[0]
	if got.Class != "off" || got.Text != "off" || got.Percentage != 0 {
		t.Fatalf("status = %+v, want off", got)
	}
	if !strings.Contains(got.Tooltip, "Desk: off") {
		t.Fatalf("tooltip = %q, want the room's lights", got.Tooltip)
	}
}

func TestWaybarStaysValidJSONWhenBridgeFails(t *testing.T) {
	withClient(t, newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))

	out, err := execute(t, "waybar")
	if err != nil {
		t.Fatalf("execute() error = %v, want nil so Waybar keeps the module", err)
	}
	got := decodeWaybarLines(t, strings.SplitN(out, "\nerror:", 2)[0])[0]
	if got.Class != "disconnected" || !strings.Contains(got.Tooltip, "boom") {
		t.Fatalf("status = %+v, want disconnected with the error in the tooltip", got)
	}
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestWatchWaybarRerendersOnEvents(t *testing.T) {
	var mu sync.Mutex
	homeOn := true
	release := make(chan struct{})
	bridge := newRecordingBridge(t, waybarGets(true))
	client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/eventstream/clip/v2" {
			w.(http.Flusher).Flush()
			<-release
			mu.Lock()
			homeOn = false
			mu.Unlock()
			_, _ = fmt.Fprintf(w, "data: [{\"creationtime\":\"2026-10-24T10:00:00Z\",\"id\":\"e1\",\"type\":\"update\",\"data\":[{\"id\":%q,\"type\":\"grouped_light\",\"on\":{\"on\":false}}]}]\n\n", homeGroupedID)
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			return
		}
		mu.Lock()
		bridge.gets = waybarGets(homeOn)
		mu.Unlock()
		bridge.handle(w, r)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out := &syncBuffer{}
	done := make(chan error, 1)
	go func() {
		done <- watchWaybar(ctx, client, waybarSelection{}, &waybarWriter{w: out}, &bytes.Buffer{})
	}()

	waitFor(t, func() bool { return strings.Count(out.String(), "\n") >= 1 })
	close(release)
	waitFor(t, func() bool { return strings.Contains(out.String(), `"class":"off"`) })
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("watchWaybar() error = %v", err)
	}

	statuses := decodeWaybarLines(t, out.String())
	if statuses[0].Class != "on" || statuses[len(statuses)-1].Class != "off" {
		t.Fatalf("statuses = %+v, want on then off", statuses)
	}
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met within 5s")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
