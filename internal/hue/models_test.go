package hue

import "testing"

func TestLightAccessors(t *testing.T) {
	l := Light{ID: "1", Metadata: NamedMetadata{Name: "Desk Lamp"}, On: OnState{On: true}}
	if l.Name() != "Desk Lamp" || l.GetName() != "Desk Lamp" || l.GetID() != "1" || !l.IsOn() {
		t.Fatalf("Light accessors = %+v, want name/GetName Desk Lamp, GetID 1, IsOn true", l)
	}
}

func TestGroupAccessors(t *testing.T) {
	g := Group{ID: "room-1", Metadata: NamedMetadata{Name: "Kitchen"}}
	if g.Name() != "Kitchen" || g.GetName() != "Kitchen" || g.GetID() != "room-1" {
		t.Fatalf("Group accessors = %+v, want name/GetName Kitchen, GetID room-1", g)
	}
}

func TestSceneAccessors(t *testing.T) {
	s := Scene{ID: "scene-1", Metadata: NamedMetadata{Name: "Movie Night"}}
	if s.Name() != "Movie Night" || s.GetName() != "Movie Night" || s.GetID() != "scene-1" {
		t.Fatalf("Scene accessors = %+v, want name/GetName Movie Night, GetID scene-1", s)
	}
}

func TestLightCapabilities(t *testing.T) {
	plain := Light{}
	if plain.SupportsColor() || plain.SupportsTemperature() || plain.SupportsDimming() || plain.SupportedEffects() != nil {
		t.Errorf("plain on/off light reports capabilities: %+v", plain)
	}
	full := Light{
		Color:            &ColorState{},
		ColorTemperature: &ColorTemperatureState{},
		Dimming:          &DimmingState{},
		Effects:          &EffectsState{Status: "candle", EffectValues: []string{EffectNone, "candle"}},
	}
	if !full.SupportsColor() || !full.SupportsTemperature() || !full.SupportsDimming() {
		t.Errorf("full-color light misses a capability: %+v", full)
	}
	if full.ActiveEffect() != "candle" || len(full.SupportedEffects()) != 1 {
		t.Errorf("effects = %q, %v", full.ActiveEffect(), full.SupportedEffects())
	}
}

func TestOtherAccessors(t *testing.T) {
	if (Device{ID: "d"}).GetID() != "d" || (behaviorScript{ID: "s"}).GetID() != "s" {
		t.Error("GetID")
	}
	smart := SmartScene{Metadata: NamedMetadata{Name: "Natural light"}}
	if smart.QualifiedName() != "Natural light" {
		t.Errorf("QualifiedName without a group = %q", smart.QualifiedName())
	}
	smart.GroupName = "Office"
	if smart.QualifiedName() != "Office/Natural light" {
		t.Errorf("QualifiedName = %q", smart.QualifiedName())
	}
}
