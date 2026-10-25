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

func sceneGroup(s *Scene) (string, *string) { return s.Group.RID, &s.GroupName }

// ListWithGroupNames lists scenes with GroupName set to the room or zone each
// scene belongs to.
func (n *Scenes) ListWithGroupNames(ctx context.Context) ([]Scene, error) {
	return listWithGroupNames(ctx, n.resourceNamespace, sceneGroup)
}

// FindByName accepts a plain scene name or a "<room or zone>/<scene>"
// qualified name.
func (n *Scenes) FindByName(ctx context.Context, name string) (Scene, error) {
	return findGrouped(ctx, n.resourceNamespace, name, sceneGroup)
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
	if activation.Transition != nil && *activation.Transition < 0 {
		return APIResponse[ResourceIdentifier]{}, fmt.Errorf("transition must not be negative")
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

// groupedResource is a resource that belongs to a room or zone and can be
// addressed as "<room or zone>/<name>".
type groupedResource interface {
	named
	QualifiedName() string
}

// groupSlot returns the room or zone ID a resource belongs to and where to
// store that group's name.
type groupSlot[T any] func(*T) (groupID string, groupName *string)

func (c *Client) groupNames(ctx context.Context) (map[string]string, error) {
	names := map[string]string{}
	for _, groups := range []*groupNamespace{&c.Rooms.groupNamespace, &c.Zones.groupNamespace} {
		list, err := groups.List(ctx)
		if err != nil {
			return nil, err
		}
		for _, g := range list {
			names[g.ID] = g.Name()
		}
	}
	return names, nil
}

func fillGroupNames[T any](ctx context.Context, c *Client, items []T, slot groupSlot[T]) error {
	names, err := c.groupNames(ctx)
	if err != nil {
		return err
	}
	for i := range items {
		id, name := slot(&items[i])
		*name = names[id]
	}
	return nil
}

func listWithGroupNames[T named](ctx context.Context, r resourceNamespace[T], slot groupSlot[T]) ([]T, error) {
	items, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	if err := fillGroupNames(ctx, r.client, items, slot); err != nil {
		return nil, err
	}
	return items, nil
}

// findGrouped matches a plain or a "<room or zone>/<name>" qualified name.
// Room and zone names are only fetched when the plain name is missing or
// ambiguous, to keep the common case to one request.
func findGrouped[T groupedResource](ctx context.Context, r resourceNamespace[T], name string, slot groupSlot[T]) (T, error) {
	var zero T
	items, err := r.List(ctx)
	if err != nil {
		return zero, err
	}
	item, plainErr := matchName(r.resourceType, items, name, T.GetName)
	if plainErr == nil {
		return item, nil
	}

	if err := fillGroupNames(ctx, r.client, items, slot); err != nil {
		return zero, err
	}
	item, qualifiedErr := matchName(r.resourceType, items, name, T.QualifiedName)
	if qualifiedErr == nil {
		return item, nil
	}

	var ambiguous *AmbiguousNameError
	if errors.As(plainErr, &ambiguous) {
		ambiguous.Candidates = ambiguous.Candidates[:0]
		wanted := normalizeName(name)
		for _, item := range items {
			if normalizeName(item.GetName()) == wanted {
				ambiguous.Candidates = append(ambiguous.Candidates, item.QualifiedName())
			}
		}
		return zero, ambiguous
	}
	return zero, qualifiedErr
}
