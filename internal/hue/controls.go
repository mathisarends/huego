package hue

import (
	"context"
	"fmt"
)

const (
	defaultBrightnessStep = 10.0
	identifyAlertAction   = "breathe"
)

// Controllable is implemented by anything a light command can target: a
// single light, or a room/zone acting through its grouped_light service.
type Controllable interface {
	Apply(ctx context.Context, id string, update LightUpdate) (APIResponse[ResourceIdentifier], error)
	IsOn(ctx context.Context, id string) (bool, error)
}

type StateChange struct {
	On         *bool
	Brightness *float64
	Color      *ColorXY
	Kelvin     *int
	Mirek      *int
	Transition *float64 // seconds
}

func buildLightUpdate(change StateChange) (LightUpdate, error) {
	if change.Kelvin != nil && change.Mirek != nil {
		return LightUpdate{}, fmt.Errorf("pass either kelvin or mirek, not both")
	}
	if change.Color != nil && (change.Kelvin != nil || change.Mirek != nil) {
		return LightUpdate{}, fmt.Errorf("a light shows either a color or a color temperature")
	}

	mirek := change.Mirek
	if change.Kelvin != nil {
		m, err := KelvinToMirek(*change.Kelvin)
		if err != nil {
			return LightUpdate{}, err
		}
		mirek = &m
	}

	update := LightUpdate{}
	if change.On != nil {
		update.On = &OnState{On: *change.On}
	}
	if change.Brightness != nil {
		update.Dimming = &DimmingState{Brightness: *change.Brightness}
	}
	if change.Color != nil {
		update.Color = &ColorState{XY: change.Color}
	}
	if mirek != nil {
		update.ColorTemperature = &ColorTemperatureState{Mirek: mirek}
	}
	if change.Transition != nil {
		ms := transitionToMilliseconds(*change.Transition)
		update.Dynamics = &DynamicsState{Duration: &ms}
	}
	return update, nil
}

func transitionToMilliseconds(seconds float64) int {
	return int(seconds*1000 + 0.5)
}

func SetState(ctx context.Context, target Controllable, id string, change StateChange) (APIResponse[ResourceIdentifier], error) {
	update, err := buildLightUpdate(change)
	if err != nil {
		return APIResponse[ResourceIdentifier]{}, err
	}
	return target.Apply(ctx, id, update)
}

func TurnOn(ctx context.Context, target Controllable, id string, brightness *float64, kelvin *int, transition *float64) (APIResponse[ResourceIdentifier], error) {
	on := true
	return SetState(ctx, target, id, StateChange{On: &on, Brightness: brightness, Kelvin: kelvin, Transition: transition})
}

func TurnOff(ctx context.Context, target Controllable, id string, transition *float64) (APIResponse[ResourceIdentifier], error) {
	off := false
	return SetState(ctx, target, id, StateChange{On: &off, Transition: transition})
}

func Toggle(ctx context.Context, target Controllable, id string, transition *float64) (APIResponse[ResourceIdentifier], error) {
	currentlyOn, err := target.IsOn(ctx, id)
	if err != nil {
		return APIResponse[ResourceIdentifier]{}, err
	}
	on := !currentlyOn
	return SetState(ctx, target, id, StateChange{On: &on, Transition: transition})
}

func SetBrightness(ctx context.Context, target Controllable, id string, brightness float64, transition *float64) (APIResponse[ResourceIdentifier], error) {
	if brightness == 0 {
		return TurnOff(ctx, target, id, transition)
	}
	on := true
	return SetState(ctx, target, id, StateChange{On: &on, Brightness: &brightness, Transition: transition})
}

func Brighten(ctx context.Context, target Controllable, id string, by float64) (APIResponse[ResourceIdentifier], error) {
	if by == 0 {
		by = defaultBrightnessStep
	}
	return target.Apply(ctx, id, LightUpdate{DimmingDelta: &DimmingDelta{Action: DimmingDeltaUp, BrightnessDelta: by}})
}

func Dim(ctx context.Context, target Controllable, id string, by float64) (APIResponse[ResourceIdentifier], error) {
	if by == 0 {
		by = defaultBrightnessStep
	}
	return target.Apply(ctx, id, LightUpdate{DimmingDelta: &DimmingDelta{Action: DimmingDeltaDown, BrightnessDelta: by}})
}

func SetRGB(ctx context.Context, target Controllable, id string, rgb RGB, brightness *float64, transition *float64) (APIResponse[ResourceIdentifier], error) {
	xy := RGBToXY(rgb)
	on := true
	return SetState(ctx, target, id, StateChange{On: &on, Color: &xy, Brightness: brightness, Transition: transition})
}

func SetHex(ctx context.Context, target Controllable, id string, hexColor string, brightness *float64, transition *float64) (APIResponse[ResourceIdentifier], error) {
	rgb, err := HexToRGB(hexColor)
	if err != nil {
		return APIResponse[ResourceIdentifier]{}, err
	}
	return SetRGB(ctx, target, id, rgb, brightness, transition)
}

func SetColorTemperature(ctx context.Context, target Controllable, id string, kelvin int, brightness *float64, transition *float64) (APIResponse[ResourceIdentifier], error) {
	on := true
	return SetState(ctx, target, id, StateChange{On: &on, Kelvin: &kelvin, Brightness: brightness, Transition: transition})
}

func Identify(ctx context.Context, target Controllable, id string) (APIResponse[ResourceIdentifier], error) {
	return target.Apply(ctx, id, LightUpdate{Alert: &AlertState{Action: identifyAlertAction}})
}
