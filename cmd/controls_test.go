package cmd

import (
	"net/http"
	"strings"
	"testing"

	"github.com/mathisarends/huego/internal/hue"
)

const (
	homeID         = "11111111-1111-1111-1111-111111111111"
	homeGroupedID  = "22222222-2222-2222-2222-222222222222"
	effectLightID  = "33333333-3333-3333-3333-333333333333"
	plainLightID   = "44444444-4444-4444-4444-444444444444"
	updatedPayload = `{"data":[{"rid":"x","rtype":"light"}],"errors":[]}`
)

// recordingBridge serves fixed GET responses by path and records PUT bodies.
type recordingBridge struct {
	t     *testing.T
	gets  map[string]any
	puts  map[string]hue.LightUpdate
	order []string
}

func newRecordingBridge(t *testing.T, gets map[string]any) *recordingBridge {
	return &recordingBridge{t: t, gets: gets, puts: map[string]hue.LightUpdate{}}
}

func (b *recordingBridge) handle(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/clip/v2/resource/")
	b.order = append(b.order, r.Method+" "+path)
	switch r.Method {
	case http.MethodGet:
		body, ok := b.gets[path]
		if !ok {
			b.t.Fatalf("unexpected GET %s", path)
		}
		writeJSONFixture(b.t, w, body)
	case http.MethodPut:
		var update hue.LightUpdate
		decodeJSONFixture(b.t, r, &update)
		b.puts[path] = update
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(updatedPayload))
	default:
		b.t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	}
}

func (b *recordingBridge) use(t *testing.T) {
	withClient(t, newFakeClient(t, b.handle))
}

func homeGets() map[string]any {
	return map[string]any{
		"bridge_home": hue.APIResponse[hue.Group]{Data: []hue.Group{{
			ID:       homeID,
			Type:     hue.ResourceTypeBridgeHome,
			Services: []hue.ResourceReference{{RID: homeGroupedID, RType: hue.ResourceTypeGroupedLight}},
		}}},
		"grouped_light": hue.APIResponse[hue.GroupedLight]{Data: []hue.GroupedLight{{
			ID: homeGroupedID, On: &hue.OnState{On: true}, Dimming: &hue.DimmingState{Brightness: 40},
		}}},
		"light": hue.APIResponse[hue.Light]{Data: []hue.Light{
			{ID: plainLightID, Metadata: hue.NamedMetadata{Name: "Desk"}, On: hue.OnState{On: true}},
		}},
	}
}

func TestHomeOffSwitchesBridgeHomeGroupedLight(t *testing.T) {
	bridge := newRecordingBridge(t, homeGets())
	bridge.use(t)

	if _, err := execute(t, "home", "off", "-t", "2"); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	update, ok := bridge.puts["grouped_light/"+homeGroupedID]
	if !ok {
		t.Fatalf("no PUT to the home grouped_light; requests: %v", bridge.order)
	}
	if update.On == nil || update.On.On {
		t.Fatalf("On = %+v, want off", update.On)
	}
	if update.Dynamics == nil || update.Dynamics.Duration == nil || *update.Dynamics.Duration != 2000 {
		t.Fatalf("Dynamics = %+v, want 2000ms fade", update.Dynamics)
	}
}

func TestHomeRejectsTargetArgument(t *testing.T) {
	newRecordingBridge(t, homeGets()).use(t)

	if _, err := execute(t, "home", "on", "Kitchen"); err == nil {
		t.Fatal("execute() error = nil, want an argument count error")
	}
}

func TestHomeGetShowsAggregateState(t *testing.T) {
	newRecordingBridge(t, homeGets()).use(t)

	out, err := execute(t, "home", "get")
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	for _, want := range []string{"Home", "on", "40%", "Desk"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output = %q, want it to contain %q", out, want)
		}
	}
}

func TestDimAndBrightenSendRelativeSteps(t *testing.T) {
	tests := []struct {
		args       []string
		wantAction hue.DimmingDeltaAction
		wantDelta  float64
		wantOn     bool
	}{
		{[]string{"home", "dim"}, hue.DimmingDeltaDown, 10, false},
		{[]string{"home", "down", "25"}, hue.DimmingDeltaDown, 25, false},
		{[]string{"home", "brighten"}, hue.DimmingDeltaUp, 10, true},
		{[]string{"home", "up", "5"}, hue.DimmingDeltaUp, 5, true},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			bridge := newRecordingBridge(t, homeGets())
			bridge.use(t)

			if _, err := execute(t, tt.args...); err != nil {
				t.Fatalf("execute() error = %v", err)
			}
			update := bridge.puts["grouped_light/"+homeGroupedID]
			if update.DimmingDelta == nil || update.DimmingDelta.Action != tt.wantAction || update.DimmingDelta.BrightnessDelta != tt.wantDelta {
				t.Fatalf("DimmingDelta = %+v, want %s by %v", update.DimmingDelta, tt.wantAction, tt.wantDelta)
			}
			if gotOn := update.On != nil && update.On.On; gotOn != tt.wantOn {
				t.Fatalf("turns on = %v, want %v", gotOn, tt.wantOn)
			}
		})
	}
}

func TestControlInputIsValidatedBeforeAnyRequest(t *testing.T) {
	tests := [][]string{
		{"home", "dim", "0"},
		{"home", "dim", "150"},
		{"home", "brightness", "abc"},
		{"home", "color", "not-a-color"},
		{"home", "temperature", "100"},
		{"home", "on", "--transition", "-1"},
		{"home", "on", "--brightness", "101"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			bridge := newRecordingBridge(t, homeGets())
			bridge.use(t)

			if _, err := execute(t, args...); err == nil {
				t.Fatal("execute() error = nil, want a validation error")
			}
			if len(bridge.order) != 0 {
				t.Fatalf("requests = %v, want none", bridge.order)
			}
		})
	}
}

func effectGets() map[string]any {
	return map[string]any{
		"light": hue.APIResponse[hue.Light]{Data: []hue.Light{{
			ID: effectLightID, Metadata: hue.NamedMetadata{Name: "Candle"},
			Effects: &hue.EffectsState{Status: hue.EffectNone, EffectValues: []string{hue.EffectNone, "candle", "fire"}},
		}}},
		"light/" + effectLightID: hue.APIResponse[hue.Light]{Data: []hue.Light{{
			ID: effectLightID, Metadata: hue.NamedMetadata{Name: "Candle"},
			Effects: &hue.EffectsState{Status: hue.EffectNone, EffectValues: []string{hue.EffectNone, "candle", "fire"}},
		}}},
	}
}

func TestLightEffect(t *testing.T) {
	tests := []struct {
		effect string
		want   string
	}{
		{"fire", "fire"},
		{"none", hue.EffectNone},
	}
	for _, tt := range tests {
		t.Run(tt.effect, func(t *testing.T) {
			bridge := newRecordingBridge(t, effectGets())
			bridge.use(t)

			if _, err := execute(t, "light", "effect", "Candle", tt.effect); err != nil {
				t.Fatalf("execute() error = %v", err)
			}
			update := bridge.puts["light/"+effectLightID]
			if update.Effects == nil || update.Effects.Effect != tt.want {
				t.Fatalf("Effects = %+v, want effect %q", update.Effects, tt.want)
			}
		})
	}
}

func TestLightEffectRejectsUnsupportedEffect(t *testing.T) {
	bridge := newRecordingBridge(t, effectGets())
	bridge.use(t)

	_, err := execute(t, "light", "effect", "Candle", "sparkle")
	if err == nil || !strings.Contains(err.Error(), "candle, fire") {
		t.Fatalf("error = %v, want it to list the supported effects", err)
	}
	if len(bridge.puts) != 0 {
		t.Fatalf("puts = %v, want none", bridge.puts)
	}
}
