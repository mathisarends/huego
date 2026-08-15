package hue

import "context"

type Scenes struct {
	resourceNamespace[Scene]
}

func newScenes(client *Client) *Scenes {
	return &Scenes{resourceNamespace[Scene]{client: client, resourceType: "scene"}}
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
