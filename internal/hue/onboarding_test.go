package hue

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegisterAppKeySucceedsOnFirstPoll(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api" {
			t.Errorf("path = %q, want /api", r.URL.Path)
		}
		writeJSONFixture(t, w, []map[string]any{
			{"success": map[string]string{"username": "new-app-key", "clientkey": "abc123"}},
		})
	}))
	defer srv.Close()

	app, err := RegisterAppKey(t.Context(), strings.TrimPrefix(srv.URL, "https://"))
	if err != nil {
		t.Fatalf("RegisterAppKey() error = %v", err)
	}
	if app.AppKey != "new-app-key" || app.ClientKey != "abc123" {
		t.Fatalf("RegisterAppKey() = %+v, want app key + client key", app)
	}
}

func TestRegisterAppKeyRetriesUntilLinkButtonPressed(t *testing.T) {
	attempts := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			writeJSONFixture(t, w, []map[string]any{
				{"error": map[string]string{"description": "link button not pressed"}},
			})
			return
		}
		writeJSONFixture(t, w, []map[string]any{
			{"success": map[string]string{"username": "new-app-key"}},
		})
	}))
	defer srv.Close()

	registrationPollInterval = 0 // don't slow the test down waiting for a real link-button press
	app, err := RegisterAppKey(t.Context(), strings.TrimPrefix(srv.URL, "https://"))
	if err != nil {
		t.Fatalf("RegisterAppKey() error = %v", err)
	}
	if attempts < 2 {
		t.Fatalf("attempts = %d, want at least 2", attempts)
	}
	if app.AppKey != "new-app-key" {
		t.Fatalf("AppKey = %q, want new-app-key", app.AppKey)
	}
}
