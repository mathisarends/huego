package hue

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestEventsDecodeMultilineAndResume(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/eventstream/clip/v2" || r.Header.Get("Last-Event-ID") != "old" || r.Header.Get("Accept") != "text/event-stream" {
			t.Errorf("invalid event request: %v", r)
		}
		_, _ = io.WriteString(w, ": keepalive\r\nid: next\r\ndata: [\r\ndata: {\"type\":\"update\",\"data\":[]}\r\ndata: ]\r\n\r\n")
	})
	var messages []EventMessage
	err := client.Events(t.Context(), "old", func(message EventMessage) error { messages = append(messages, message); return nil })
	if !errors.Is(err, io.EOF) || len(messages) != 1 || messages[0].ID != "next" {
		t.Fatalf("messages=%v err=%v", messages, err)
	}
}

func TestEventsInvalidAndInterruptedFrames(t *testing.T) {
	if err := readEvents(strings.NewReader("data: invalid\n\n"), "", func(EventMessage) error { return nil }); err == nil || errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
	calls := 0
	_ = readEvents(strings.NewReader("data: []\n"), "", func(EventMessage) error { calls++; return nil })
	if calls != 0 {
		t.Fatal("incomplete frame emitted")
	}
}
