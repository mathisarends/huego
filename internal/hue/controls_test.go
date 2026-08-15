package hue

import (
	"context"
	"errors"
	"testing"
)

func TestBuildLightUpdateRejectsKelvinAndMirek(t *testing.T) {
	kelvin, mirek := 2700, 300
	_, err := buildLightUpdate(StateChange{Kelvin: &kelvin, Mirek: &mirek})
	if err == nil {
		t.Fatal("buildLightUpdate() error = nil, want error")
	}
}

func TestBuildLightUpdateRejectsColorAndKelvin(t *testing.T) {
	kelvin := 2700
	color := ColorXY{X: 0.3, Y: 0.3}
	_, err := buildLightUpdate(StateChange{Color: &color, Kelvin: &kelvin})
	if err == nil {
		t.Fatal("buildLightUpdate() error = nil, want error")
	}
}

func TestBuildLightUpdateSetsMirekFromKelvin(t *testing.T) {
	kelvin := 2700
	update, err := buildLightUpdate(StateChange{Kelvin: &kelvin})
	if err != nil {
		t.Fatalf("buildLightUpdate() error = %v", err)
	}
	if update.ColorTemperature == nil || *update.ColorTemperature.Mirek != 370 {
		t.Fatalf("ColorTemperature = %+v, want mirek 370", update.ColorTemperature)
	}
}

func TestBuildLightUpdateSetsTransitionDuration(t *testing.T) {
	transition := 1.5
	update, err := buildLightUpdate(StateChange{Transition: &transition})
	if err != nil {
		t.Fatalf("buildLightUpdate() error = %v", err)
	}
	if update.Dynamics == nil || *update.Dynamics.Duration != 1500 {
		t.Fatalf("Dynamics = %+v, want duration 1500", update.Dynamics)
	}
}

func TestBuildLightUpdateRejectsInvalidKelvin(t *testing.T) {
	kelvin := 0
	if _, err := buildLightUpdate(StateChange{Kelvin: &kelvin}); err == nil {
		t.Fatal("buildLightUpdate() error = nil, want error")
	}
}

// fakeControllable is an in-memory stand-in for a light or grouped_light,
// letting the pure command functions in controls.go be tested without an
// HTTP server.
type fakeControllable struct {
	on         bool
	applyErr   error
	isOnErr    error
	lastID     string
	lastUpdate LightUpdate
	applyCalls int
}

func (f *fakeControllable) Apply(_ context.Context, id string, update LightUpdate) (APIResponse[ResourceIdentifier], error) {
	f.applyCalls++
	f.lastID = id
	f.lastUpdate = update
	if f.applyErr != nil {
		return APIResponse[ResourceIdentifier]{}, f.applyErr
	}
	if update.On != nil {
		f.on = update.On.On
	}
	return APIResponse[ResourceIdentifier]{Data: []ResourceIdentifier{{RID: id, RType: ResourceTypeLight}}}, nil
}

func (f *fakeControllable) IsOn(_ context.Context, _ string) (bool, error) {
	if f.isOnErr != nil {
		return false, f.isOnErr
	}
	return f.on, nil
}

func TestToggleFlipsCurrentState(t *testing.T) {
	target := &fakeControllable{on: false}
	if _, err := Toggle(t.Context(), target, "abc", nil); err != nil {
		t.Fatalf("Toggle() error = %v", err)
	}
	if !target.on {
		t.Fatal("Toggle() left light off, want on")
	}

	if _, err := Toggle(t.Context(), target, "abc", nil); err != nil {
		t.Fatalf("Toggle() error = %v", err)
	}
	if target.on {
		t.Fatal("Toggle() left light on, want off")
	}
}

func TestToggleReturnsIsOnError(t *testing.T) {
	wantErr := errors.New("bridge unreachable")
	target := &fakeControllable{isOnErr: wantErr}
	if _, err := Toggle(t.Context(), target, "abc", nil); !errors.Is(err, wantErr) {
		t.Fatalf("Toggle() error = %v, want %v", err, wantErr)
	}
}

func TestSetBrightnessZeroTurnsOff(t *testing.T) {
	target := &fakeControllable{on: true}
	if _, err := SetBrightness(t.Context(), target, "abc", 0, nil); err != nil {
		t.Fatalf("SetBrightness() error = %v", err)
	}
	if target.on {
		t.Fatal("SetBrightness(0) left light on, want off")
	}
	if target.lastUpdate.Dimming != nil {
		t.Fatalf("Dimming = %+v, want nil for a zero-brightness turn-off", target.lastUpdate.Dimming)
	}
}

func TestSetBrightnessPositiveTurnsOn(t *testing.T) {
	target := &fakeControllable{}
	if _, err := SetBrightness(t.Context(), target, "abc", 42, nil); err != nil {
		t.Fatalf("SetBrightness() error = %v", err)
	}
	if !target.on {
		t.Fatal("SetBrightness(42) left light off, want on")
	}
	if target.lastUpdate.Dimming == nil || target.lastUpdate.Dimming.Brightness != 42 {
		t.Fatalf("Dimming = %+v, want brightness 42", target.lastUpdate.Dimming)
	}
}

func TestBrightenDefaultsStep(t *testing.T) {
	target := &fakeControllable{}
	if _, err := Brighten(t.Context(), target, "abc", 0); err != nil {
		t.Fatalf("Brighten() error = %v", err)
	}
	delta := target.lastUpdate.DimmingDelta
	if delta == nil || delta.Action != DimmingDeltaUp || delta.BrightnessDelta != defaultBrightnessStep {
		t.Fatalf("DimmingDelta = %+v, want up %v", delta, defaultBrightnessStep)
	}
}

func TestBrightenCustomStep(t *testing.T) {
	target := &fakeControllable{}
	if _, err := Brighten(t.Context(), target, "abc", 25); err != nil {
		t.Fatalf("Brighten() error = %v", err)
	}
	delta := target.lastUpdate.DimmingDelta
	if delta == nil || delta.BrightnessDelta != 25 {
		t.Fatalf("DimmingDelta = %+v, want delta 25", delta)
	}
}

func TestDimDefaultsStep(t *testing.T) {
	target := &fakeControllable{}
	if _, err := Dim(t.Context(), target, "abc", 0); err != nil {
		t.Fatalf("Dim() error = %v", err)
	}
	delta := target.lastUpdate.DimmingDelta
	if delta == nil || delta.Action != DimmingDeltaDown || delta.BrightnessDelta != defaultBrightnessStep {
		t.Fatalf("DimmingDelta = %+v, want down %v", delta, defaultBrightnessStep)
	}
}

func TestSetRGBConvertsToXY(t *testing.T) {
	target := &fakeControllable{}
	if _, err := SetRGB(t.Context(), target, "abc", RGB{255, 0, 0}, nil, nil); err != nil {
		t.Fatalf("SetRGB() error = %v", err)
	}
	if target.lastUpdate.Color == nil || target.lastUpdate.Color.XY == nil {
		t.Fatalf("Color = %+v, want an XY value", target.lastUpdate.Color)
	}
	if !target.on {
		t.Fatal("SetRGB() left light off, want on")
	}
}

func TestSetHexInvalidColor(t *testing.T) {
	target := &fakeControllable{}
	if _, err := SetHex(t.Context(), target, "abc", "not-a-color", nil, nil); err == nil {
		t.Fatal("SetHex() error = nil, want error")
	}
	if target.applyCalls != 0 {
		t.Fatalf("Apply() called %d times, want 0 for an invalid color", target.applyCalls)
	}
}

func TestSetHexValidColor(t *testing.T) {
	target := &fakeControllable{}
	if _, err := SetHex(t.Context(), target, "abc", "#ff0000", nil, nil); err != nil {
		t.Fatalf("SetHex() error = %v", err)
	}
	if target.lastUpdate.Color == nil {
		t.Fatalf("Color = %+v, want a value set from the hex string", target.lastUpdate.Color)
	}
}

func TestSetColorTemperatureSetsKelvin(t *testing.T) {
	target := &fakeControllable{}
	if _, err := SetColorTemperature(t.Context(), target, "abc", 2700, nil, nil); err != nil {
		t.Fatalf("SetColorTemperature() error = %v", err)
	}
	if target.lastUpdate.ColorTemperature == nil || *target.lastUpdate.ColorTemperature.Mirek != 370 {
		t.Fatalf("ColorTemperature = %+v, want mirek 370", target.lastUpdate.ColorTemperature)
	}
	if !target.on {
		t.Fatal("SetColorTemperature() left light off, want on")
	}
}

func TestIdentifySendsBreatheAlert(t *testing.T) {
	target := &fakeControllable{}
	if _, err := Identify(t.Context(), target, "abc"); err != nil {
		t.Fatalf("Identify() error = %v", err)
	}
	if target.lastUpdate.Alert == nil || target.lastUpdate.Alert.Action != identifyAlertAction {
		t.Fatalf("Alert = %+v, want action %q", target.lastUpdate.Alert, identifyAlertAction)
	}
}

func TestTransitionToMilliseconds(t *testing.T) {
	if got := transitionToMilliseconds(1.5); got != 1500 {
		t.Fatalf("transitionToMilliseconds(1.5) = %d, want 1500", got)
	}
	if got := transitionToMilliseconds(0); got != 0 {
		t.Fatalf("transitionToMilliseconds(0) = %d, want 0", got)
	}
}
