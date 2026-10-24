package hue

import (
	"context"
	"errors"
	"fmt"
)

type Scenes struct {
	resourceNamespace[Scene]
}

func newScenes(client *Client) *Scenes {
	return &Scenes{resourceNamespace[Scene]{client: client, resourceType: "scene"}}
}

// ListWithGroupNames lists scenes with GroupName set to the room or zone each
// scene belongs to.
func (n *Scenes) ListWithGroupNames(ctx context.Context) ([]Scene, error) {
	scenes, err := n.List(ctx)
	if err != nil {
		return nil, err
	}
	if err := n.fillGroupNames(ctx, scenes); err != nil {
		return nil, err
	}
	return scenes, nil
}

func (n *Scenes) fillGroupNames(ctx context.Context, scenes []Scene) error {
	names := map[string]string{}
	for _, groups := range []*groupNamespace{&n.client.Rooms.groupNamespace, &n.client.Zones.groupNamespace} {
		list, err := groups.List(ctx)
		if err != nil {
			return err
		}
		for _, g := range list {
			names[g.ID] = g.Name()
		}
	}
	for i := range scenes {
		scenes[i].GroupName = names[scenes[i].Group.RID]
	}
	return nil
}

// FindByName accepts a plain scene name or a "<room or zone>/<scene>"
// qualified name. Room and zone names are only fetched when the plain name
// is missing or ambiguous, to keep the common case to one request.
func (n *Scenes) FindByName(ctx context.Context, name string) (Scene, error) {
	scenes, err := n.List(ctx)
	if err != nil {
		return Scene{}, err
	}
	scene, plainErr := matchName(n.resourceType, scenes, name, Scene.GetName)
	if plainErr == nil {
		return scene, nil
	}

	if err := n.fillGroupNames(ctx, scenes); err != nil {
		return Scene{}, err
	}
	scene, qualifiedErr := matchName(n.resourceType, scenes, name, Scene.QualifiedName)
	if qualifiedErr == nil {
		return scene, nil
	}

	var ambiguous *AmbiguousNameError
	if errors.As(plainErr, &ambiguous) {
		ambiguous.Candidates = ambiguous.Candidates[:0]
		wanted := normalizeName(name)
		for _, s := range scenes {
			if normalizeName(s.Name()) == wanted {
				ambiguous.Candidates = append(ambiguous.Candidates, s.QualifiedName())
			}
		}
		return Scene{}, ambiguous
	}
	return Scene{}, qualifiedErr
}

func (n *Scenes) ResolveTarget(ctx context.Context, target string) (string, error) {
	if isUUID(target) {
		return target, nil
	}
	scene, err := n.FindByName(ctx, target)
	if err != nil {
		return "", err
	}
	return scene.ID, nil
}

func (n *Scenes) Recall(ctx context.Context, id string, request SceneRecallRequest) (APIResponse[ResourceIdentifier], error) {
	var resp APIResponse[ResourceIdentifier]
	err := n.client.put(ctx, "scene/"+id, request, &resp)
	return resp, err
}

type SceneActivation struct {
	Brightness *float64
	Transition *float64 // seconds
	Dynamic    bool
}

func (n *Scenes) Activate(ctx context.Context, id string, activation SceneActivation) (APIResponse[ResourceIdentifier], error) {
	if activation.Brightness != nil && (*activation.Brightness < 0 || *activation.Brightness > 100) {
		return APIResponse[ResourceIdentifier]{}, fmt.Errorf("brightness must be between 0 and 100")
	}
	action := SceneActionActive
	if activation.Dynamic {
		action = SceneActionDynamicPalette
	}

	recall := SceneRecall{Action: action}
	if activation.Transition != nil {
		ms := transitionToMilliseconds(*activation.Transition)
		recall.Duration = &ms
	}
	if activation.Brightness != nil {
		recall.Dimming = &DimmingState{Brightness: *activation.Brightness}
	}

	return n.Recall(ctx, id, SceneRecallRequest{Recall: recall})
}
