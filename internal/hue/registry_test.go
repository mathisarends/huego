package hue

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestCatalogMatchesPinnedContract(t *testing.T) {
	data, err := os.ReadFile("testdata/clip_v2_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var contract APICatalog
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatal(err)
	}
	catalog := Catalog()
	if len(catalog.Resources) != 41 || catalog.Revision != contract.Revision {
		t.Fatalf("unexpected catalog size or revision: %d, %s", len(catalog.Resources), catalog.Revision)
	}
	for i, capability := range catalog.Resources {
		capability.RequestSchemas = nil
		if !reflect.DeepEqual(capability, contract.Resources[i]) {
			t.Errorf("capability %d = %+v, want %+v", i, capability, contract.Resources[i])
		}
	}
}

func TestNativeResourceContract(t *testing.T) {
	for _, capability := range Catalog().Resources {
		t.Run(capability.ResourceType, func(t *testing.T) {
			for _, item := range []bool{false, true} {
				for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
					id := ""
					if item {
						id = "01234567-89ab-cdef-0123-456789abcdef"
					}
					calls := 0
					client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
						calls++
						path := basePath + "/" + capability.ResourceType
						if item {
							path += "/" + id
						}
						if r.URL.Path != path || r.Method != method || r.Header.Get("hue-application-key") != "test-app-key" {
							t.Errorf("request = %s %s (%s)", r.Method, r.URL.Path, r.Header.Get("hue-application-key"))
						}
						body, _ := io.ReadAll(r.Body)
						if method == "GET" || method == "DELETE" {
							if len(body) != 0 {
								t.Errorf("unexpected body: %s", body)
							}
						} else if string(body) != `{"enabled":false,"future":null}` || r.Header.Get("Content-Type") != "application/json" {
							t.Errorf("body = %s", body)
						}
						_, _ = fmt.Fprintf(w, `{"errors":[],"data":[{"id":%q,"type":%q,"future":{"number":9007199254740993,"value":null}}]}`, id, capability.ResourceType)
					})
					namespace, err := client.Resources.Of(capability.ResourceType, false)
					if err != nil {
						t.Fatal(err)
					}
					var response ResourceResponse
					if method == "GET" {
						if item {
							response, err = namespace.Get(t.Context(), id)
						} else {
							response, err = namespace.List(t.Context())
						}
					} else {
						var body json.RawMessage
						if method != "DELETE" {
							body = json.RawMessage(`{"enabled":false,"future":null}`)
						}
						response, err = namespace.Write(t.Context(), method, id, body)
					}
					if !capability.Supports(method, item) {
						if err == nil || calls != 0 {
							t.Errorf("unsupported %s item=%t: err=%v calls=%d", method, item, err, calls)
						}
						continue
					}
					if err != nil || calls != 1 || len(response.Data) != 1 {
						t.Fatalf("%s item=%t: response=%+v err=%v calls=%d", method, item, response, err, calls)
					}
					if string(response.Data[0]["future"]) != `{"number":9007199254740993,"value":null}` {
						t.Fatalf("unknown fields lost: %s", response.Data[0]["future"])
					}
				}
			}
		})
	}
}

func TestResourceRootAndRawExtensions(t *testing.T) {
	var paths []string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		_, _ = io.WriteString(w, `{"errors":[],"data":[]}`)
	})
	if _, err := client.Resources.List(t.Context()); err != nil {
		t.Fatal(err)
	}
	raw, err := client.Resources.Of("future_service", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Write(t.Context(), "POST", "", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Resources.Of("future_service", false); err == nil {
		t.Fatal("unknown checked type accepted")
	}
	if !reflect.DeepEqual(paths, []string{"GET /clip/v2/resource", "POST /clip/v2/resource/future_service"}) {
		t.Fatal(paths)
	}
}

func TestNativeRejectsInvalidInputBeforeNetwork(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid input reached the bridge")
	})
	for _, kind := range []string{"", "../light", "light?x=1", "LIGHT", "light/foo"} {
		if _, err := client.Resources.Of(kind, true); err == nil {
			t.Errorf("accepted kind %q", kind)
		}
	}
	namespace, _ := client.Resources.Of("room", true)
	for _, id := range []string{"", "../abc", "abc?x=1", "abc/def", "abc#fragment"} {
		if _, err := namespace.Get(t.Context(), id); err == nil {
			t.Errorf("accepted ID %q", id)
		}
	}
	for _, body := range []string{"", "null", "[]", "1", `"text"`, `{`, `{} {}`} {
		if _, err := namespace.Write(t.Context(), "PUT", "abc", json.RawMessage(body)); err == nil {
			t.Errorf("accepted body %q", body)
		}
	}
	if _, err := namespace.Write(t.Context(), "PATCH", "abc", json.RawMessage(`{}`)); err == nil {
		t.Fatal("accepted PATCH")
	}
	if _, err := namespace.Write(t.Context(), "DELETE", "abc", json.RawMessage(`{}`)); err == nil {
		t.Fatal("accepted DELETE body")
	}
}

func TestNativeTargetResolution(t *testing.T) {
	for _, test := range []struct {
		target string
		data   string
		want   string
		err    string
	}{
		{" desk ", `[{"id":"a","metadata":{"name":"Desk"}},{"id":"b"}]`, "a", ""},
		{"routine", `[{"id":"a","name":"Routine"}]`, "a", ""},
		{"desk", `[{"id":"a","metadata":{"name":"Desk"}},{"id":"b","metadata":{"name":"DESK"}}]`, "", "ambiguous"},
		{"des", `[{"id":"a","metadata":{"name":"Desk"}}]`, "", "Known names: Desk"},
		{" ", `[{"id":"a"}]`, "", "non-empty"},
		{"desk", `[{"id":"../escape","metadata":{"name":"Desk"}}]`, "", "invalid resource ID"},
	} {
		t.Run(test.target+test.err, func(t *testing.T) {
			client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = fmt.Fprintf(w, `{"errors":[],"data":%s}`, test.data)
			})
			namespace, _ := client.Resources.Of("device", false)
			id, err := namespace.ResolveTarget(t.Context(), test.target)
			if id != test.want || (test.err == "" && err != nil) || (test.err != "" && (err == nil || !strings.Contains(err.Error(), test.err))) {
				t.Fatalf("id=%q err=%v", id, err)
			}
		})
	}
}

func TestNativeBridgeErrorsRetainDetails(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"errors":[{"description":"disabled","code":123}],"data":[]}`)
	})
	namespace, _ := client.Resources.Of("device", false)
	response, err := namespace.List(t.Context())
	if err != nil || response.Err() == nil || !strings.Contains(response.Err().Error(), "disabled") || !strings.Contains(string(response.Errors[0]), `"code":123`) {
		t.Fatalf("response=%+v err=%v", response, err)
	}
	if _, err := namespace.ResolveTarget(t.Context(), "Desk"); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("ResolveTarget error=%v", err)
	}
	if _, err := client.Lights.List(t.Context()); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("typed List error=%v", err)
	}
}
