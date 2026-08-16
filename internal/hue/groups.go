package hue

import "context"

// groupNamespace is the shared behaviour of the room and zone endpoints:
// light commands address the group's grouped_light service, so a room or
// zone is switched in one bridge call instead of light by light.
type groupNamespace struct {
	resourceNamespace[Group]
}

func (n *groupNamespace) ListWithState(ctx context.Context) ([]Group, error) {
	groups, err := n.List(ctx)
	if err != nil {
		return nil, err
	}

	var resp APIResponse[GroupedLight]
	if err := n.client.get(ctx, "grouped_light", &resp); err != nil {
		return nil, err
	}
	states := make(map[string]*OnState, len(resp.Data))
	for i := range resp.Data {
		states[resp.Data[i].ID] = resp.Data[i].On
	}
	for i := range groups {
		for _, service := range groups[i].Services {
			if service.RType == ResourceTypeGroupedLight {
				groups[i].AggregateOn = states[service.RID]
				break
			}
		}
	}
	return groups, nil
}

func (n *groupNamespace) groupedLightID(ctx context.Context, id string) (string, error) {
	group, err := n.GetOne(ctx, id)
	if err != nil {
		return "", err
	}
	for _, service := range group.Services {
		if service.RType == ResourceTypeGroupedLight {
			return service.RID, nil
		}
	}
	return "", newResourceNotFoundError("%s %s has no grouped light service", n.resourceType, id)
}

func (n *groupNamespace) groupedLight(ctx context.Context, id string) (GroupedLight, error) {
	groupedLightID, err := n.groupedLightID(ctx, id)
	if err != nil {
		return GroupedLight{}, err
	}
	var resp APIResponse[GroupedLight]
	if err := n.client.get(ctx, "grouped_light/"+groupedLightID, &resp); err != nil {
		return GroupedLight{}, err
	}
	if len(resp.Data) == 0 {
		return GroupedLight{}, newResourceNotFoundError("no grouped light with ID %s", groupedLightID)
	}
	return resp.Data[0], nil
}

func (n *groupNamespace) Apply(ctx context.Context, id string, update LightUpdate) (APIResponse[ResourceIdentifier], error) {
	groupedLightID, err := n.groupedLightID(ctx, id)
	if err != nil {
		return APIResponse[ResourceIdentifier]{}, err
	}
	var resp APIResponse[ResourceIdentifier]
	err = n.client.put(ctx, "grouped_light/"+groupedLightID, update, &resp)
	return resp, err
}

func (n *groupNamespace) IsOn(ctx context.Context, id string) (bool, error) {
	groupedLight, err := n.groupedLight(ctx, id)
	if err != nil {
		return false, err
	}
	return groupedLight.On != nil && groupedLight.On.On, nil
}

type Rooms struct{ groupNamespace }

func newRooms(client *Client) *Rooms {
	return &Rooms{groupNamespace{resourceNamespace[Group]{client: client, resourceType: "room"}}}
}

type Zones struct{ groupNamespace }

func newZones(client *Client) *Zones {
	return &Zones{groupNamespace{resourceNamespace[Group]{client: client, resourceType: "zone"}}}
}
