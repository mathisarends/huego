package hue

import "context"

type Lights struct {
	resourceNamespace[Light]
}

func newLights(client *Client) *Lights {
	return &Lights{resourceNamespace[Light]{client: client, resourceType: "light"}}
}

func (n *Lights) Apply(ctx context.Context, id string, update LightUpdate) (APIResponse[ResourceIdentifier], error) {
	var resp APIResponse[ResourceIdentifier]
	err := n.client.put(ctx, "light/"+id, update, &resp)
	return resp, err
}

func (n *Lights) IsOn(ctx context.Context, id string) (bool, error) {
	light, err := n.GetOne(ctx, id)
	if err != nil {
		return false, err
	}
	return light.On.On, nil
}

// ListWithRooms lists lights with RoomName set to the room each light's
// device is assigned to.
func (n *Lights) ListWithRooms(ctx context.Context) ([]Light, error) {
	lights, err := n.List(ctx)
	if err != nil {
		return nil, err
	}
	rooms, err := n.client.Rooms.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range lights {
		for _, room := range rooms {
			if room.Contains(lights[i]) {
				lights[i].RoomName = room.Name()
				break
			}
		}
	}
	return lights, nil
}

// SetEffect starts a light effect such as "candle" or "fire"; EffectNone
// stops it. Effects exist only on individual lights, not on grouped_light.
func (n *Lights) SetEffect(ctx context.Context, id, effect string) (APIResponse[ResourceIdentifier], error) {
	return n.Apply(ctx, id, LightUpdate{Effects: &EffectsState{Effect: effect}})
}
