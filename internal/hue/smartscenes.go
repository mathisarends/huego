package hue

import "context"

const ResourceTypeSmartScene ResourceType = "smart_scene"

type SmartSceneState string

const (
	SmartSceneActive   SmartSceneState = "active"
	SmartSceneInactive SmartSceneState = "inactive"
)

type SmartSceneTimeslot struct {
	TimeslotID int    `json:"timeslot_id"`
	Weekday    string `json:"weekday"`
}

// SmartScene switches between scenes by time of day, like the Hue app's
// "Natural light".
type SmartScene struct {
	ID             string              `json:"id"`
	Metadata       NamedMetadata       `json:"metadata"`
	Group          ResourceReference   `json:"group"`
	State          SmartSceneState     `json:"state"`
	ActiveTimeslot *SmartSceneTimeslot `json:"active_timeslot,omitempty"`
	GroupName      string              `json:"-"`
}

func (s SmartScene) GetID() string   { return s.ID }
func (s SmartScene) GetName() string { return s.Metadata.Name }

func (s SmartScene) QualifiedName() string {
	if s.GroupName == "" {
		return s.Metadata.Name
	}
	return s.GroupName + "/" + s.Metadata.Name
}

type SmartScenes struct {
	resourceNamespace[SmartScene]
}

func newSmartScenes(client *Client) *SmartScenes {
	return &SmartScenes{resourceNamespace[SmartScene]{client: client, resourceType: string(ResourceTypeSmartScene)}}
}

func smartSceneGroup(s *SmartScene) (string, *string) { return s.Group.RID, &s.GroupName }

func (n *SmartScenes) ListWithGroupNames(ctx context.Context) ([]SmartScene, error) {
	return listWithGroupNames(ctx, n.resourceNamespace, smartSceneGroup)
}

// FindByName accepts a plain smart scene name or a "<room or zone>/<name>"
// qualified name.
func (n *SmartScenes) FindByName(ctx context.Context, name string) (SmartScene, error) {
	return findGrouped(ctx, n.resourceNamespace, name, smartSceneGroup)
}

func (n *SmartScenes) ResolveTarget(ctx context.Context, target string) (string, error) {
	if isUUID(target) {
		return target, nil
	}
	scene, err := n.FindByName(ctx, target)
	if err != nil {
		return "", err
	}
	return scene.ID, nil
}

// SetActive starts a smart scene, or stops it and leaves its lights as they are.
func (n *SmartScenes) SetActive(ctx context.Context, id string, active bool) (APIResponse[ResourceIdentifier], error) {
	action := "deactivate"
	if active {
		action = "activate"
	}
	body := map[string]any{"recall": map[string]string{"action": action}}
	var resp APIResponse[ResourceIdentifier]
	err := n.client.put(ctx, n.resourceType+"/"+id, body, &resp)
	return resp, err
}
