package hue

// ResourceType identifies the kind of a resource or reference on the bridge.
// Only the subset huego works with is enumerated; the bridge reports many more.
type ResourceType string

const (
	ResourceTypeLight        ResourceType = "light"
	ResourceTypeGroupedLight ResourceType = "grouped_light"
	ResourceTypeRoom         ResourceType = "room"
	ResourceTypeZone         ResourceType = "zone"
	ResourceTypeScene        ResourceType = "scene"
	ResourceTypeDevice       ResourceType = "device"
	ResourceTypeBridgeHome   ResourceType = "bridge_home"
)

type ResourceReference struct {
	RID   string       `json:"rid"`
	RType ResourceType `json:"rtype"`
}

type ResourceIdentifier = ResourceReference

type ResponseError struct {
	Description string `json:"description"`
}

type APIResponse[T any] struct {
	Errors []ResponseError `json:"errors"`
	Data   []T             `json:"data"`
}

type NamedMetadata struct {
	Name      string `json:"name"`
	Archetype string `json:"archetype,omitempty"`
}

type OnState struct {
	On bool `json:"on"`
}

type DimmingState struct {
	Brightness  float64  `json:"brightness"`
	MinDimLevel *float64 `json:"min_dim_level,omitempty"`
}

type DimmingDeltaAction string

const (
	DimmingDeltaUp   DimmingDeltaAction = "up"
	DimmingDeltaDown DimmingDeltaAction = "down"
	DimmingDeltaStop DimmingDeltaAction = "stop"
)

type DimmingDelta struct {
	Action          DimmingDeltaAction `json:"action"`
	BrightnessDelta float64            `json:"brightness_delta,omitempty"`
}

type MirekSchema struct {
	Minimum int `json:"mirek_minimum"`
	Maximum int `json:"mirek_maximum"`
}

type ColorTemperatureState struct {
	Mirek       *int         `json:"mirek,omitempty"`
	MirekValid  bool         `json:"mirek_valid,omitempty"`
	MirekSchema *MirekSchema `json:"mirek_schema,omitempty"`
}

type ColorXY struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type ColorState struct {
	XY        *ColorXY `json:"xy,omitempty"`
	GamutType string   `json:"gamut_type,omitempty"`
}

type DynamicsState struct {
	Duration *int `json:"duration,omitempty"`
}

type AlertState struct {
	Action string `json:"action,omitempty"`
}

// EffectNone stops a running effect.
const EffectNone = "no_effect"

type EffectsState struct {
	Effect       string   `json:"effect,omitempty"`
	Status       string   `json:"status,omitempty"`
	EffectValues []string `json:"effect_values,omitempty"`
}

type Light struct {
	ID               string                 `json:"id"`
	Metadata         NamedMetadata          `json:"metadata"`
	Owner            ResourceReference      `json:"owner"`
	On               OnState                `json:"on"`
	Dimming          *DimmingState          `json:"dimming,omitempty"`
	ColorTemperature *ColorTemperatureState `json:"color_temperature,omitempty"`
	Color            *ColorState            `json:"color,omitempty"`
	Effects          *EffectsState          `json:"effects,omitempty"`
	RoomName         string                 `json:"-"`
}

func (l Light) Name() string    { return l.Metadata.Name }
func (l Light) IsOn() bool      { return l.On.On }
func (l Light) GetID() string   { return l.ID }
func (l Light) GetName() string { return l.Metadata.Name }

func (l Light) Brightness() *float64 {
	if l.Dimming == nil {
		return nil
	}
	brightness := l.Dimming.Brightness
	return &brightness
}

// Kelvin is the current color temperature, or nil while the light shows an
// xy color (the bridge then reports mirek_valid=false).
func (l Light) Kelvin() *int {
	ct := l.ColorTemperature
	if ct == nil || !ct.MirekValid || ct.Mirek == nil {
		return nil
	}
	kelvin, err := MirekToKelvin(*ct.Mirek)
	if err != nil {
		return nil
	}
	return &kelvin
}

// Hex is the current xy color as #rrggbb at full brightness, or "" while
// the light is in color temperature mode or cannot show color.
func (l Light) Hex() string {
	if l.Kelvin() != nil || l.Color == nil || l.Color.XY == nil {
		return ""
	}
	return XYToHex(*l.Color.XY, 100)
}

// KelvinRange is the supported color temperature range, or nil for lights
// without color temperature support.
func (l Light) KelvinRange() *[2]int {
	ct := l.ColorTemperature
	if ct == nil || ct.MirekSchema == nil || ct.MirekSchema.Minimum <= 0 || ct.MirekSchema.Maximum <= 0 {
		return nil
	}
	warmest, _ := MirekToKelvin(ct.MirekSchema.Maximum)
	coolest, _ := MirekToKelvin(ct.MirekSchema.Minimum)
	return &[2]int{warmest, coolest}
}

func (l Light) SupportsColor() bool { return l.Color != nil }

func (l Light) SupportsTemperature() bool { return l.ColorTemperature != nil }

func (l Light) SupportsDimming() bool { return l.Dimming != nil }

// ActiveEffect is the running effect, or "" when none is active.
func (l Light) ActiveEffect() string {
	if l.Effects == nil || l.Effects.Status == EffectNone {
		return ""
	}
	return l.Effects.Status
}

// SupportedEffects lists the effect names the light accepts, without no_effect.
func (l Light) SupportedEffects() []string {
	if l.Effects == nil {
		return nil
	}
	effects := make([]string, 0, len(l.Effects.EffectValues))
	for _, effect := range l.Effects.EffectValues {
		if effect != EffectNone {
			effects = append(effects, effect)
		}
	}
	return effects
}

// LightUpdate is a partial update: only non-nil fields are sent to the bridge.
type LightUpdate struct {
	On               *OnState               `json:"on,omitempty"`
	Dimming          *DimmingState          `json:"dimming,omitempty"`
	DimmingDelta     *DimmingDelta          `json:"dimming_delta,omitempty"`
	ColorTemperature *ColorTemperatureState `json:"color_temperature,omitempty"`
	Color            *ColorState            `json:"color,omitempty"`
	Dynamics         *DynamicsState         `json:"dynamics,omitempty"`
	Alert            *AlertState            `json:"alert,omitempty"`
	Effects          *EffectsState          `json:"effects,omitempty"`
}

type Group struct {
	ID       string              `json:"id"`
	Metadata NamedMetadata       `json:"metadata"`
	Children []ResourceReference `json:"children"`
	Services []ResourceReference `json:"services"`
	Type     ResourceType        `json:"type"`
	State    *GroupedLight       `json:"-"`
	Lights   []Light             `json:"-"`
}

func (g Group) Name() string    { return g.Metadata.Name }
func (g Group) GetID() string   { return g.ID }
func (g Group) GetName() string { return g.Metadata.Name }

// IsOn reports the aggregate state: a group is on while any light in it is.
func (g Group) IsOn() bool { return g.State != nil && g.State.On != nil && g.State.On.On }

// Brightness is the bridge's aggregate brightness over the group's lights
// that are on, or nil when no state has been loaded.
func (g Group) Brightness() *float64 {
	if g.State == nil || g.State.Dimming == nil {
		return nil
	}
	brightness := g.State.Dimming.Brightness
	return &brightness
}

// Contains reports whether a light belongs to the group. Rooms list devices
// as children and zones list light services, so both are checked.
func (g Group) Contains(light Light) bool {
	for _, child := range g.Children {
		switch child.RType {
		case ResourceTypeLight:
			if child.RID == light.ID {
				return true
			}
		case ResourceTypeDevice:
			if child.RID == light.Owner.RID {
				return true
			}
		}
	}
	return false
}

func (g Group) groupedLightID() string {
	for _, service := range g.Services {
		if service.RType == ResourceTypeGroupedLight {
			return service.RID
		}
	}
	return ""
}

type GroupedLight struct {
	ID               string                 `json:"id"`
	Owner            *ResourceReference     `json:"owner,omitempty"`
	On               *OnState               `json:"on,omitempty"`
	Dimming          *DimmingState          `json:"dimming,omitempty"`
	ColorTemperature *ColorTemperatureState `json:"color_temperature,omitempty"`
	Color            *ColorState            `json:"color,omitempty"`
}

type SceneStatusValue string

const (
	SceneStatusActive         SceneStatusValue = "active"
	SceneStatusInactive       SceneStatusValue = "inactive"
	SceneStatusStatic         SceneStatusValue = "static"
	SceneStatusDynamicPalette SceneStatusValue = "dynamic_palette"
)

type SceneStatus struct {
	Active *SceneStatusValue `json:"active,omitempty"`
}

type Scene struct {
	ID        string            `json:"id"`
	Metadata  NamedMetadata     `json:"metadata"`
	Group     ResourceReference `json:"group"`
	Status    *SceneStatus      `json:"status,omitempty"`
	GroupName string            `json:"-"`
}

func (s Scene) Name() string { return s.Metadata.Name }

// QualifiedName is "<room or zone>/<scene>", which stays unique when several
// rooms share a scene name. It falls back to the plain name until GroupName
// has been filled in.
func (s Scene) QualifiedName() string {
	if s.GroupName == "" {
		return s.Metadata.Name
	}
	return s.GroupName + "/" + s.Metadata.Name
}

func (s Scene) GetID() string   { return s.ID }
func (s Scene) GetName() string { return s.Metadata.Name }

type SceneAction string

const (
	SceneActionActive         SceneAction = "active"
	SceneActionDynamicPalette SceneAction = "dynamic_palette"
	SceneActionStatic         SceneAction = "static"
)

type SceneRecall struct {
	Action   SceneAction   `json:"action"`
	Duration *int          `json:"duration,omitempty"`
	Dimming  *DimmingState `json:"dimming,omitempty"`
}

type SceneRecallRequest struct {
	Recall SceneRecall `json:"recall"`
}
