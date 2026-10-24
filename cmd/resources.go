package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

type capabilities struct {
	Dimming     bool     `json:"dimming"`
	Color       bool     `json:"color"`
	Temperature *[2]int  `json:"temperature_kelvin,omitempty"`
	Effects     []string `json:"effects,omitempty"`
}

type lightView struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Room         string       `json:"room,omitempty"`
	Archetype    string       `json:"archetype,omitempty"`
	On           bool         `json:"on"`
	Brightness   *float64     `json:"brightness,omitempty"`
	Kelvin       *int         `json:"kelvin,omitempty"`
	Color        string       `json:"color,omitempty"`
	Effect       string       `json:"effect,omitempty"`
	Capabilities capabilities `json:"capabilities"`
}

func newLightView(l hue.Light) lightView {
	return lightView{
		ID:         l.ID,
		Name:       l.Name(),
		Room:       l.RoomName,
		Archetype:  l.Metadata.Archetype,
		On:         l.IsOn(),
		Brightness: l.Brightness(),
		Kelvin:     l.Kelvin(),
		Color:      l.Hex(),
		Effect:     l.ActiveEffect(),
		Capabilities: capabilities{
			Dimming:     l.SupportsDimming(),
			Color:       l.SupportsColor(),
			Temperature: l.KelvinRange(),
			Effects:     l.SupportedEffects(),
		},
	}
}

var lightListing = listing[lightView]{
	headers: []string{"NAME", "ROOM", "STATE", "BRIGHTNESS", "COLOR", "ID"},
	row: func(v lightView) []string {
		return []string{v.Name, orDash(v.Room), onOff(v.On), percent(v.Brightness), v.colorText(), v.ID}
	},
	name: func(v lightView) string { return v.Name },
}

func (v lightView) colorText() string {
	switch {
	case v.Kelvin != nil:
		return strconv.Itoa(*v.Kelvin) + "K"
	case v.Color != "":
		return v.Color
	default:
		return "-"
	}
}

func (v lightView) fields() []field {
	supports := []string{}
	if v.Capabilities.Dimming {
		supports = append(supports, "dimming")
	}
	if v.Capabilities.Color {
		supports = append(supports, "color")
	}
	if r := v.Capabilities.Temperature; r != nil {
		supports = append(supports, fmt.Sprintf("temperature %d-%dK", r[0], r[1]))
	}
	return []field{
		{"Name", v.Name},
		{"ID", v.ID},
		{"Room", v.Room},
		{"Type", v.Archetype},
		{"State", onOff(v.On)},
		{"Brightness", percent(v.Brightness)},
		{"Color", v.colorText()},
		{"Effect", v.Effect},
		{"Supports", strings.Join(supports, ", ")},
		{"Effects", strings.Join(v.Capabilities.Effects, ", ")},
	}
}

type groupLight struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	On   bool   `json:"on"`
}

type groupView struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	On         bool         `json:"on"`
	Brightness *float64     `json:"brightness,omitempty"`
	Lights     []groupLight `json:"lights"`
}

func newGroupView(g hue.Group) groupView {
	lights := make([]groupLight, len(g.Lights))
	for i, l := range g.Lights {
		lights[i] = groupLight{ID: l.ID, Name: l.Name(), On: l.IsOn()}
	}
	return groupView{ID: g.ID, Name: g.Name(), On: g.IsOn(), Brightness: g.Brightness(), Lights: lights}
}

func (v groupView) lightsOn() int {
	on := 0
	for _, l := range v.Lights {
		if l.On {
			on++
		}
	}
	return on
}

var groupListing = listing[groupView]{
	headers: []string{"NAME", "STATE", "BRIGHTNESS", "LIGHTS ON", "ID"},
	row: func(v groupView) []string {
		return []string{v.Name, onOff(v.On), percent(v.Brightness), fmt.Sprintf("%d/%d", v.lightsOn(), len(v.Lights)), v.ID}
	},
	name: func(v groupView) string { return v.Name },
}

func (v groupView) fields() []field {
	lights := make([]string, len(v.Lights))
	for i, l := range v.Lights {
		lights[i] = fmt.Sprintf("%s (%s)", l.Name, onOff(l.On))
	}
	return []field{
		{"Name", v.Name},
		{"ID", v.ID},
		{"State", onOff(v.On)},
		{"Brightness", percent(v.Brightness)},
		{"Lights", strings.Join(lights, ", ")},
	}
}

type sceneView struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Group  string `json:"group,omitempty"`
	Target string `json:"target"`
	Status string `json:"status,omitempty"`
}

func newSceneView(s hue.Scene) sceneView {
	status := ""
	if s.Status != nil && s.Status.Active != nil {
		status = string(*s.Status.Active)
	}
	return sceneView{ID: s.ID, Name: s.Name(), Group: s.GroupName, Target: s.QualifiedName(), Status: status}
}

var sceneListing = listing[sceneView]{
	headers: []string{"NAME", "GROUP", "STATUS", "ID"},
	row: func(v sceneView) []string {
		return []string{v.Name, orDash(v.Group), orDash(v.Status), v.ID}
	},
	name: func(v sceneView) string { return v.Target },
}

func (v sceneView) fields() []field {
	return []field{
		{"Name", v.Name},
		{"ID", v.ID},
		{"Group", v.Group},
		{"Status", v.Status},
	}
}

func mapViews[T, V any](items []T, view func(T) V) []V {
	views := make([]V, len(items))
	for i, item := range items {
		views[i] = view(item)
	}
	return views
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

func percent(value *float64) string {
	if value == nil {
		return "-"
	}
	return strconv.FormatFloat(*value, 'f', 0, 64) + "%"
}

func orDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func writeResponse(cmd *cobra.Command, target string, resp hue.APIResponse[hue.ResourceIdentifier]) error {
	written, err := writeJSON(cmd, resp)
	if err != nil {
		return err
	}
	if len(resp.Errors) > 0 {
		descriptions := make([]string, len(resp.Errors))
		for i, responseError := range resp.Errors {
			descriptions[i] = responseError.Description
		}
		return partialUpdateError{fmt.Sprintf("bridge could not fully update %q: %s", target, strings.Join(descriptions, "; "))}
	}
	if written {
		return nil
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Updated %q (%d resource(s)).\n", target, len(resp.Data))
	return nil
}
