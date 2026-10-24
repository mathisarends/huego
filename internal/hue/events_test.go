package hue

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const sampleEventMessage = `id: 1700000000:0
data: [{"creationtime":"2026-10-24T10:00:00Z","id":"e1","type":"update",
data: "data":[{"id":"l1","type":"light","on":{"on":true}},{"id":"g1","type":"grouped_light","dimming":{"brightness":40}}]}]

`

func TestReadEventsParsesMultiLineMessagesAndSkipsComments(t *testing.T) {
	var events []Event
	err := readEvents(strings.NewReader(": hi\n\n"+sampleEventMessage), func(e Event) { events = append(events, e) })
	if !errors.Is(err, ErrStreamClosed) {
		t.Fatalf("readEvents() error = %v, want ErrStreamClosed at end of input", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	resources := events[0].Resources()
	if len(resources) != 2 || resources[0].ID != "l1" || resources[1].Type != ResourceTypeGroupedLight {
		t.Fatalf("resources = %+v, want light l1 and grouped_light g1", resources)
	}
	if !strings.Contains(string(resources[1].Raw), `"brightness":40`) {
		t.Fatalf("raw = %s, want the changed properties", resources[1].Raw)
	}
}

func TestReadEventsRejectsMalformedData(t *testing.T) {
	err := readEvents(strings.NewReader("data: not json\n\n"), func(Event) {})
	if err == nil || errors.Is(err, ErrStreamClosed) {
		t.Fatalf("readEvents() error = %v, want a decode error", err)
	}
}

func TestEventsSendsAppKeyAndStreamsEvents(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/eventstream/clip/v2" {
			t.Fatalf("path = %s, want /eventstream/clip/v2", r.URL.Path)
		}
		if r.Header.Get("hue-application-key") != "test-app-key" {
			t.Fatalf("missing app key header")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, sampleEventMessage)
	})

	var connected bool
	var events []Event
	err := client.Events(context.Background(), func() { connected = true }, func(e Event) { events = append(events, e) })
	if !errors.Is(err, ErrStreamClosed) {
		t.Fatalf("Events() error = %v, want ErrStreamClosed", err)
	}
	if !connected || len(events) != 1 {
		t.Fatalf("connected = %v, events = %d; want connected and 1 event", connected, len(events))
	}
}

func TestWatchEventsReconnectsAfterTheStreamCloses(t *testing.T) {
	var connections atomic.Int32
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		connections.Add(1)
		_, _ = fmt.Fprint(w, sampleEventMessage)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var disconnects int
	err := client.WatchEvents(ctx, EventWatch{
		Disconnected: func(err error, retryIn time.Duration) {
			disconnects++
			if disconnects == 2 {
				cancel()
			}
		},
	})
	if err != nil {
		t.Fatalf("WatchEvents() error = %v, want nil after cancel", err)
	}
	if connections.Load() < 2 {
		t.Fatalf("connections = %d, want a reconnect", connections.Load())
	}
}

func TestWatchEventsStopsOnRejectedAppKey(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized user", http.StatusForbidden)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := client.WatchEvents(ctx, EventWatch{})
	if !IsAuthError(err) {
		t.Fatalf("WatchEvents() error = %v, want an auth error", err)
	}
}
