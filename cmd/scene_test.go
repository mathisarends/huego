package cmd

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/mathisarends/huego/internal/hue"
)

func TestSceneActivateCommandJSON(t *testing.T) {
	client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSONFixture(t, w, hue.APIResponse[hue.ResourceIdentifier]{
			Data: []hue.ResourceIdentifier{{RID: "scene-1", RType: hue.ResourceTypeScene}},
		})
	})
	withClient(t, client)

	out, err := execute(t, "scene", "activate", "01234567-89ab-cdef-0123-456789abcdef", "--json")
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if !strings.Contains(out, `"rid":"scene-1"`) {
		t.Fatalf("output = %q, want JSON with rid scene-1", out)
	}
}

func TestSceneActivateCommandClientError(t *testing.T) {
	orig := newClient
	t.Cleanup(func() { newClient = orig })
	wantErr := "no bridge configured"
	newClient = func() (*hue.Client, error) { return nil, errors.New(wantErr) }

	_, err := execute(t, "scene", "activate", "01234567-89ab-cdef-0123-456789abcdef")
	if err == nil || !strings.Contains(err.Error(), wantErr) {
		t.Fatalf("execute() error = %v, want it to mention %q", err, wantErr)
	}
}

func TestSceneActivateCommand(t *testing.T) {
	var gotRequest hue.SceneRecallRequest
	client := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		decodeJSONFixture(t, r, &gotRequest)
		writeJSONFixture(t, w, hue.APIResponse[hue.ResourceIdentifier]{
			Data: []hue.ResourceIdentifier{{RID: "scene-1", RType: hue.ResourceTypeScene}},
		})
	})
	withClient(t, client)

	out, err := execute(t, "scene", "activate", "01234567-89ab-cdef-0123-456789abcdef", "--brightness", "60")
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if !strings.Contains(out, "Updated 1 resource(s).") {
		t.Fatalf("output = %q, want update confirmation", out)
	}
	if gotRequest.Recall.Dimming == nil || gotRequest.Recall.Dimming.Brightness != 60 {
		t.Fatalf("Recall.Dimming = %+v, want brightness 60", gotRequest.Recall.Dimming)
	}
}
