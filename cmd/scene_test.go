package cmd

import (
	"net/http"
	"strings"
	"testing"

	"github.com/mathisarends/huego/internal/hue"
)

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
