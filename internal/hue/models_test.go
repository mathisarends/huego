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
