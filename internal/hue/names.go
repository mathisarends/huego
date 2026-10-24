package hue

import "context"

// HomeName labels the bridge_home group, which has no name of its own.
const HomeName = "Home"

// Names maps the IDs of lights, rooms, zones, the home, their grouped_light
// services, and scenes to display names. Scenes use their qualified
// "<group>/<scene>" name, since plain scene names repeat across rooms.
func (c *Client) Names(ctx context.Context) (map[string]string, error) {
	names := map[string]string{}

	lights, err := c.Lights.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, light := range lights {
		names[light.ID] = light.Name()
	}

	for _, groups := range []*groupNamespace{&c.Rooms.groupNamespace, &c.Zones.groupNamespace, &c.Home.groupNamespace} {
		list, err := groups.List(ctx)
		if err != nil {
			return nil, err
		}
		for _, group := range list {
			name := group.Name()
			if group.Type == ResourceTypeBridgeHome {
				name = HomeName
			}
			names[group.ID] = name
			if id := group.groupedLightID(); id != "" {
				names[id] = name
			}
		}
	}

	scenes, err := c.Scenes.ListWithGroupNames(ctx)
	if err != nil {
		return nil, err
	}
	for _, scene := range scenes {
		names[scene.ID] = scene.QualifiedName()
	}
	return names, nil
}
