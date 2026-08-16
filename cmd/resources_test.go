package cmd

import (
	"testing"

	"github.com/mathisarends/huego/internal/hue"
)

func TestGroupRows(t *testing.T) {
	rows := groupRows([]hue.Group{
		{ID: "room-1", Metadata: hue.NamedMetadata{Name: "Kitchen"}, AggregateOn: &hue.OnState{On: true}},
	})
	if len(rows) != 1 || rows[0].ID != "room-1" || rows[0].Name != "Kitchen" || rows[0].State != "on" {
		t.Fatalf("groupRows() = %+v, want one row for Kitchen with state on", rows)
	}
}

func TestSceneRowsActive(t *testing.T) {
	active := hue.SceneStatusActive
	rows := sceneRows([]hue.Scene{
		{ID: "scene-1", Metadata: hue.NamedMetadata{Name: "Movie Night"}, Status: &hue.SceneStatus{Active: &active}},
	})
	if len(rows) != 1 || rows[0].State != "active" {
		t.Fatalf("sceneRows() = %+v, want state active", rows)
	}
}

func TestSceneRowsNoStatus(t *testing.T) {
	rows := sceneRows([]hue.Scene{
		{ID: "scene-1", Metadata: hue.NamedMetadata{Name: "Movie Night"}},
	})
	if len(rows) != 1 || rows[0].State != "" {
		t.Fatalf("sceneRows() = %+v, want empty state when Status is nil", rows)
	}
}
