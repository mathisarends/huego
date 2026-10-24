package cmd

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAPISendsMethodPathAndBody(t *testing.T) {
	var gotMethod, gotPath, gotBody string
	withClient(t, newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotMethod, gotPath, gotBody = r.Method, r.URL.Path, string(body)
		_, _ = w.Write([]byte(`{"errors":[],"data":[{"rid":"abc","rtype":"light"}]}`))
	}))

	out, err := execute(t, "api", "put", "/clip/v2/resource/light/abc", `{"on":{"on":true}}`)
	if err != nil {
		t.Fatalf("api: %v", err)
	}
	if gotMethod != http.MethodPut || gotPath != "/clip/v2/resource/light/abc" || gotBody != `{"on":{"on":true}}` {
		t.Fatalf("request = %s %s %s", gotMethod, gotPath, gotBody)
	}
	if !strings.Contains(out, "\n  \"data\": [") {
		t.Fatalf("output not indented:\n%s", out)
	}
}

func TestAPIJSONIsCompact(t *testing.T) {
	withClient(t, newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{\n  \"errors\": [],\n  \"data\": []\n}"))
	}))

	out, err := execute(t, "api", "get", "device", "--json")
	if err != nil {
		t.Fatalf("api: %v", err)
	}
	if out != "{\"errors\":[],\"data\":[]}\n" {
		t.Fatalf("output = %q", out)
	}
}

func TestAPIReadsBodyFromStdin(t *testing.T) {
	var gotBody string
	withClient(t, newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		_, _ = w.Write([]byte(`{"errors":[],"data":[]}`))
	}))
	rootCmd.SetIn(strings.NewReader(`{"recall":{"action":"activate"}}`))
	t.Cleanup(func() { rootCmd.SetIn(nil) })

	if _, err := execute(t, "api", "put", "smart_scene/x", "-"); err != nil {
		t.Fatalf("api: %v", err)
	}
	if gotBody != `{"recall":{"action":"activate"}}` {
		t.Fatalf("body = %q", gotBody)
	}
}

func TestAPIRejectsBadInputBeforeRequest(t *testing.T) {
	withClient(t, newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
	}))

	for _, args := range [][]string{
		{"api", "patch", "light"},
		{"api", "put", "light/x", "{not json"},
	} {
		_, err := execute(t, args...)
		if _, code := classifyError(err); code != exitUsage {
			t.Fatalf("%v: err = %v, want usage error", args, err)
		}
	}
}

func TestAPIBridgeErrorsShowDescriptions(t *testing.T) {
	withClient(t, newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errors":[{"description":"Not Found"}],"data":[]}`))
	}))

	_, err := execute(t, "api", "get", "light/missing")
	if err == nil || !strings.Contains(err.Error(), "(404): Not Found") {
		t.Fatalf("err = %v", err)
	}
	if report, code := classifyError(err); code != exitNotFound || report.Code != "not_found" {
		t.Fatalf("classified as %q/%d", report.Code, code)
	}
}
