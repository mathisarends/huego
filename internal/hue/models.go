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
)

type ResourceReference struct {
	RID   string       `json:"rid"`
	RType ResourceType `json:"rtype"`
}

type ResourceIdentifier = ResourceReference

type APIResponse[T any] struct {
	Errors []struct {
		Description string `json:"description"`
	} `json:"errors"`
	Data []T `json:"data"`
}

type NamedMetadata struct {
	Name string `json:"name"`
}

type OnState struct {
	On bool `json:"on"`
}

type DimmingState struct {
	Brightness float64 `json:"brightness"`
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

type ColorTemperatureState struct {
	Mirek      *int `json:"mirek,omitempty"`
	MirekValid bool `json:"mirek_valid,omitempty"`
}

type ColorXY struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type ColorState struct {
	XY *ColorXY `json:"xy,omitempty"`
}

type DynamicsState struct {
	Duration *int `json:"duration,omitempty"`
}

type AlertState struct {
	Action string `json:"action,omitempty"`
}

type Light struct {
	ID               string                 `json:"id"`
	Metadata         NamedMetadata          `json:"metadata"`
	Owner            ResourceReference      `json:"owner"`
	On               OnState                `json:"on"`
	Dimming          *DimmingState          `json:"dimming,omitempty"`
	ColorTemperature *ColorTemperatureState `json:"color_temperature,omitempty"`
	Color            *ColorState            `json:"color,omitempty"`
}

func (l Light) Name() string    { return l.Metadata.Name }
func (l Light) IsOn() bool      { return l.On.On }
func (l Light) GetID() string   { return l.ID }
func (l Light) GetName() string { return l.Metadata.Name }

// LightUpdate is a partial update: only non-nil fields are sent to the bridge.
type LightUpdate struct {
	On               *OnState               `json:"on,omitempty"`
	Dimming          *DimmingState          `json:"dimming,omitempty"`
	DimmingDelta     *DimmingDelta          `json:"dimming_delta,omitempty"`
	ColorTemperature *ColorTemperatureState `json:"color_temperature,omitempty"`
	Color            *ColorState            `json:"color,omitempty"`
	Dynamics         *DynamicsState         `json:"dynamics,omitempty"`
	Alert            *AlertState            `json:"alert,omitempty"`
}

type Group struct {
	ID          string              `json:"id"`
	Metadata    NamedMetadata       `json:"metadata"`
	Children    []ResourceReference `json:"children"`
	Services    []ResourceReference `json:"services"`
	AggregateOn *OnState            `json:"-"`
}

func (g Group) Name() string    { return g.Metadata.Name }
func (g Group) GetID() string   { return g.ID }
func (g Group) GetName() string { return g.Metadata.Name }

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
	ID       string            `json:"id"`
	Metadata NamedMetadata     `json:"metadata"`
	Group    ResourceReference `json:"group"`
	Status   *SceneStatus      `json:"status,omitempty"`
}

func (s Scene) Name() string    { return s.Metadata.Name }
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
