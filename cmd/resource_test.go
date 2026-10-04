package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mathisarends/huego/internal/hue"
)

func withoutCredentials(t *testing.T) {
	t.Helper()
	original := newClient
	newClient = func() (*hue.Client, error) {
		t.Error("command attempted to load credentials")
		return nil, errors.New("no credentials")
	}
	t.Cleanup(func() { newClient = original })
}

func TestResourceOfflineDiscoveryAndPlans(t *testing.T) {
	withoutCredentials(t)
	for _, args := range [][]string{
		{"resource", "types", "--json"},
		{"resource", "schema", "smart_scene", "--json"},
		{"resource", "create", "room", "--data", `{"metadata":{"name":"Office"},"children":[]}`, "--dry-run", "--json"},
		{"resource", "update", "device", testLightUUID, "--data", `{"future":null}`, "--dry-run", "--json"},
		{"resource", "delete", "scene", testLightUUID, "--dry-run", "--json"},
	} {
		out, err := execute(t, args...)
		if err != nil || !json.Valid([]byte(out)) {
			t.Fatalf("%v: output=%q err=%v", args, out, err)
		}
		if strings.Contains(strings.Join(args, " "), "dry-run") {
			var plan requestPlan
			if err := json.Unmarshal([]byte(out), &plan); err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(plan.Path, "/clip/v2/resource/") || plan.Method == "" {
				t.Fatalf("invalid plan: %+v", plan)
			}
			if plan.Method == "DELETE" && len(plan.Body) != 0 {
				t.Fatalf("DELETE plan includes a body: %s", plan.Body)
			}
		}
	}
}

func TestResourceReadsUseFullNativeEnvelope(t *testing.T) {
	for _, args := range [][]string{
		{"resource", "list", "--json"},
		{"resource", "list", "light", "--json"},
		{"resource", "get", "light", testLightUUID, "--json"},
		{"resources", "list", "future_sensor", "--raw", "--json"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, `{"errors":[],"data":[{"id":"abc","type":"light","owner":{"rid":"device-id","rtype":"device"},"future":{"n":9007199254740993}}]}`)
			})
			withClient(t, client)
			out, err := execute(t, args...)
			if err != nil || !json.Valid([]byte(out)) || !strings.Contains(out, `"n":9007199254740993`) || !strings.Contains(out, `"owner":`) {
				t.Fatalf("out=%q err=%v", out, err)
			}
		})
	}
}

func TestResourceWritesFromDataFileAndStdin(t *testing.T) {
	payload := `{"metadata":{"name":"Office"},"future":null,"n":9007199254740993}`
	filename := filepath.Join(t.TempDir(), "payload.json")
	if err := os.WriteFile(filename, append([]byte{0xef, 0xbb, 0xbf}, []byte(payload)...), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, source := range [][]string{{"--data", payload}, {"--file", filename}, {"--file", "-"}} {
		t.Run(source[0]+source[1], func(t *testing.T) {
			client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				if r.Method != "POST" || r.URL.Path != "/clip/v2/resource/room" || string(body) != payload {
					t.Errorf("request=%s %s body=%s", r.Method, r.URL.Path, body)
				}
				_, _ = io.WriteString(w, `{"errors":[],"data":[{"rid":"created-id","rtype":"room"}]}`)
			})
			withClient(t, client)
			rootCmd.SetIn(strings.NewReader(payload))
			t.Cleanup(func() { rootCmd.SetIn(nil) })
			args := append([]string{"resource", "create", "room"}, source...)
			out, err := execute(t, append(args, "--json")...)
			if err != nil || !strings.Contains(out, `"rid":"created-id"`) {
				t.Fatalf("out=%q err=%v", out, err)
			}
		})
	}
}

func TestResourceWriteNameResolutionAndNativeRoomPath(t *testing.T) {
	var calls []string
	client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		if r.Method == "GET" {
			_, _ = fmt.Fprintf(w, `{"errors":[],"data":[{"id":%q,"metadata":{"name":"Office"},"services":[{"rid":"grouped-id","rtype":"grouped_light"}]}]}`, testLightUUID)
		} else {
			_, _ = io.WriteString(w, `{"errors":[],"data":[]}`)
		}
	})
	withClient(t, client)
	if _, err := execute(t, "resource", "update", "room", "office", "--data", `{"metadata":{"name":"Study"}}`); err != nil {
		t.Fatal(err)
	}
	if strings.Join(calls, ",") != "GET /clip/v2/resource/room,PUT /clip/v2/resource/room/"+testLightUUID {
		t.Fatal(calls)
	}
	calls = nil
	if _, err := execute(t, "resource", "delete", "room", "Office", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	if strings.Join(calls, ",") != "GET /clip/v2/resource/room" {
		t.Fatalf("dry-run wrote to bridge: %v", calls)
	}
}

func TestResourceInvalidInputDoesNotLoadCredentials(t *testing.T) {
	withoutCredentials(t)
	for _, args := range [][]string{
		{"resource", "create", "light", "--data", `{}`},
		{"resource", "update", "device_power", testLightUUID, "--data", `{}`},
		{"resource", "create", "room"},
		{"resource", "create", "room", "--data", `{}`, "--file", "-"},
		{"resource", "create", "room", "--data", `[]`},
		{"resource", "create", "room", "--data", `null`},
		{"resource", "create", "room", "--data", `{} {}`},
		{"resource", "create", "room", "--data", "{"},
		{"resource", "create", "room", "--file", "nonexistent.json"},
		{"resource", "list", "../light", "--raw"},
		{"resource", "list", "unknown"},
		{"resource", "update", "room", " ", "--data", `{}`},
	} {
		if _, err := execute(t, args...); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
}

func TestResourceErrorsAndEmptyItems(t *testing.T) {
	for _, body := range []string{
		`{"errors":[{"description":"read failed","code":7}],"data":[]}`,
		`{"errors":[],"data":[]}`,
	} {
		client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, body)
		})
		withClient(t, client)
		out, err := execute(t, "resource", "get", "light", testLightUUID, "--json")
		if err == nil || !json.Valid([]byte(out)) {
			t.Fatalf("out=%q err=%v", out, err)
		}
	}
}

func TestResourceAmbiguousWriteDoesNotMutate(t *testing.T) {
	client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Error("ambiguous target was mutated")
		}
		_, _ = io.WriteString(w, `{"errors":[],"data":[{"id":"a","metadata":{"name":"Desk"}},{"id":"b","metadata":{"name":"Desk"}}]}`)
	})
	withClient(t, client)
	if _, err := execute(t, "resource", "delete", "scene", "Desk"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatal(err)
	}
}

func TestResourcePayloadSizeLimit(t *testing.T) {
	withoutCredentials(t)
	rootCmd.SetIn(bytes.NewReader(bytes.Repeat([]byte(" "), maxPayloadBytes+1)))
	t.Cleanup(func() { rootCmd.SetIn(nil) })
	if _, err := execute(t, "resource", "create", "room", "--file", "-"); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatal(err)
	}
}
