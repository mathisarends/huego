package cmd

import (
	"testing"

	"github.com/mathisarends/huego/internal/hue"
)

func TestNewGroupViewCountsLightsOn(t *testing.T) {
	view := newGroupView(hue.Group{
		ID:       "room-1",
		Metadata: hue.NamedMetadata{Name: "Kitchen"},
		State:    &hue.GroupedLight{On: &hue.OnState{On: true}, Dimming: &hue.DimmingState{Brightness: 40}},
		Lights: []hue.Light{
			{ID: "l1", On: hue.OnState{On: true}},
			{ID: "l2"},
		},
	})
	if !view.On || *view.Brightness != 40 || view.lightsOn() != 1 || len(view.Lights) != 2 {
		t.Fatalf("newGroupView() = %+v, want on at 40%% with 1/2 lights on", view)
	}
}

func TestNewSceneViewStatus(t *testing.T) {
	active := hue.SceneStatusActive
	view := newSceneView(hue.Scene{
		ID: "scene-1", Metadata: hue.NamedMetadata{Name: "Relax"}, GroupName: "Kitchen",
		Status: &hue.SceneStatus{Active: &active},
	})
	if view.Status != "active" || view.Target != "Kitchen/Relax" {
		t.Fatalf("newSceneView() = %+v, want active Kitchen/Relax", view)
	}
}

func TestNewSceneViewNoStatus(t *testing.T) {
	view := newSceneView(hue.Scene{ID: "scene-1", Metadata: hue.NamedMetadata{Name: "Relax"}})
	if view.Status != "" || view.Target != "Relax" {
		t.Fatalf("newSceneView() = %+v, want no status and plain target", view)
	}
}

func TestNewLightViewColorModes(t *testing.T) {
	mirek := 370
	temperature := newLightView(hue.Light{
		ColorTemperature: &hue.ColorTemperatureState{
			Mirek: &mirek, MirekValid: true,
			MirekSchema: &hue.MirekSchema{Minimum: 153, Maximum: 500},
		},
		Color: &hue.ColorState{XY: &hue.ColorXY{X: 0.3, Y: 0.3}},
	})
	if temperature.colorText() != "2703K" || temperature.Color != "" {
		t.Fatalf("colorText() = %q, color = %q; want 2703K and no hex", temperature.colorText(), temperature.Color)
	}
	if r := temperature.Capabilities.Temperature; r == nil || r[0] != 2000 || r[1] != 6536 {
		t.Fatalf("Temperature capability = %v, want [2000 6536]", r)
	}

	color := newLightView(hue.Light{Color: &hue.ColorState{XY: &hue.ColorXY{X: 0.6915, Y: 0.3083}}})
	if color.colorText() == "-" || color.Kelvin != nil {
		t.Fatalf("colorText() = %q, want a hex color", color.colorText())
	}

	plain := newLightView(hue.Light{})
	if plain.colorText() != "-" {
		t.Fatalf("colorText() = %q, want -", plain.colorText())
	}
}

func TestNewLightViewEffects(t *testing.T) {
	view := newLightView(hue.Light{Effects: &hue.EffectsState{
		Status:       "candle",
		EffectValues: []string{hue.EffectNone, "candle", "fire"},
	}})
	if view.Effect != "candle" || len(view.Capabilities.Effects) != 2 {
		t.Fatalf("newLightView() = %+v, want candle active and two effects", view)
	}
}
