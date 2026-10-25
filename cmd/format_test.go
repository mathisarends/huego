package cmd

import (
	"testing"

	"github.com/mathisarends/huego/internal/hue"
)

func TestDetailTexts(t *testing.T) {
	level, off := 12, false
	tests := []struct{ got, want string }{
		{deviceView{}.batteryText(), "-"},
		{deviceView{Battery: &batteryView{State: "critical"}}.batteryText(), "critical"},
		{deviceView{Battery: &batteryView{State: "normal", Level: &level}}.batteryText(), "12%"},
		{deviceView{Battery: &batteryView{State: "low", Level: &level}}.batteryText(), "12% (low)"},
		{sensorView{}.enabledText(), "-"},
		{sensorView{Enabled: &off}.enabledText(), "no"},
		{sensorView{Type: "light_level", Value: 3.5, Unit: "lux", Enabled: &off}.summary(), "light level 3.5 lux (disabled)"},
		{sensorView{Value: "short_release", Button: 2}.valueText(), "button 2: short_release"},
		{changedText(""), "-"},
		{changedText("yesterday"), "yesterday"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("got %q, want %q", tt.got, tt.want)
		}
	}
}

func TestFindByIDReportsNotFound(t *testing.T) {
	lights := []hue.Light{{ID: "a"}}
	if light, err := findByID(lights, "a", "light"); err != nil || light.ID != "a" {
		t.Errorf("findByID(a) = %+v, %v", light, err)
	}
	if _, err := findByID(lights, "b", "light"); err == nil {
		t.Error("findByID(b): want not found")
	} else if _, code := classifyError(err); code != exitNotFound {
		t.Errorf("findByID(b) exit = %d, want %d", code, exitNotFound)
	}
}
