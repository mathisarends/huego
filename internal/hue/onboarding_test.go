package hue

import (
	"context"
	"errors"
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
				{"error": map[string]any{"type": 101, "description": "link button not pressed"}},
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

func withDiscovery(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	orig := discoveryURL
	t.Cleanup(func() { discoveryURL = orig })
	discoveryURL = srv.URL
}

func TestDiscoverBridges(t *testing.T) {
	withDiscovery(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"ecb5fafffe000000","internalipaddress":"192.168.1.20","port":443}]`))
	})

	bridges, err := DiscoverBridges(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(bridges) != 1 || bridges[0].InternalIPAddress != "192.168.1.20" {
		t.Errorf("bridges = %+v", bridges)
	}
}

func TestDiscoverBridgesFailures(t *testing.T) {
	tests := map[string]struct {
		status int
		body   string
		want   string
	}{
		"rate limited": {http.StatusTooManyRequests, "", "HTTP 429"},
		"none found":   {http.StatusOK, "[]", "no Hue bridge found"},
		"bad JSON":     {http.StatusOK, "<html>", "invalid character"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			withDiscovery(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			})
			if _, err := DiscoverBridges(t.Context()); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want %q", err, tt.want)
			}
		})
	}

	discoveryURL = "http://127.0.0.1:1"
	if _, err := DiscoverBridges(t.Context()); err == nil || !strings.Contains(err.Error(), "--ip") {
		t.Errorf("unreachable discovery: err = %v, want a hint to pass --ip", err)
	}
}

func TestRegisterAppKeyFailures(t *testing.T) {
	refused := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"error":{"type":7,"description":"invalid value for devicetype"}}]`))
	}))
	defer refused.Close()
	if _, err := RegisterAppKey(t.Context(), strings.TrimPrefix(refused.URL, "https://")); err == nil || !strings.Contains(err.Error(), "invalid value for devicetype") {
		t.Errorf("refused: err = %v", err)
	}

	garbled := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer garbled.Close()
	if _, err := RegisterAppKey(t.Context(), strings.TrimPrefix(garbled.URL, "https://")); err == nil {
		t.Error("garbled response: want error")
	}

	if _, err := RegisterAppKey(t.Context(), "127.0.0.1:1"); err == nil || !strings.Contains(err.Error(), "cannot reach") {
		t.Errorf("unreachable: err = %v", err)
	}

	waiting := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"error":{"type":101,"description":"link button not pressed"}}]`))
	}))
	defer waiting.Close()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := RegisterAppKey(ctx, strings.TrimPrefix(waiting.URL, "https://")); !errors.Is(err, context.Canceled) {
		t.Errorf("canceled: err = %v", err)
	}
}

func TestDeviceTypeFitsTheBridgeLimit(t *testing.T) {
	name := deviceType()
	if !strings.HasPrefix(name, "huego#") || len(name) > len("huego#")+19 {
		t.Errorf("deviceType() = %q", name)
	}
}
