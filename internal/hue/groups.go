package hue

import (
	"context"
	"sync"
)

// groupNamespace is the shared behaviour of the room, zone, and bridge_home
// endpoints: light commands address the group's grouped_light service, so a
// group is switched in one bridge call instead of light by light.
type groupNamespace struct {
	resourceNamespace[Group]

	// groupedLightIDs caches group ID -> grouped_light ID. The mapping never
	// changes for a group, and skipping the lookup saves a round trip on
	// every command, which keybind-driven use notices.
	mu              sync.Mutex
	groupedLightIDs map[string]string
}

func newGroupNamespace(client *Client, resourceType ResourceType) groupNamespace {
	return groupNamespace{
		resourceNamespace: resourceNamespace[Group]{client: client, resourceType: string(resourceType)},
		groupedLightIDs:   map[string]string{},
	}
}

func (n *groupNamespace) List(ctx context.Context) ([]Group, error) {
	groups, err := n.resourceNamespace.List(ctx)
	if err != nil {
		return nil, err
	}
	n.remember(groups)
	return groups, nil
}

func (n *groupNamespace) FindByName(ctx context.Context, name string) (Group, error) {
	groups, err := n.List(ctx)
	if err != nil {
		return Group{}, err
	}
	return matchName(n.resourceType, groups, name, Group.GetName)
}

func (n *groupNamespace) ResolveTarget(ctx context.Context, target string) (string, error) {
	if isUUID(target) {
		return target, nil
	}
	group, err := n.FindByName(ctx, target)
	if err != nil {
		return "", err
	}
	return group.ID, nil
}

func (n *groupNamespace) remember(groups []Group) {
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, g := range groups {
		if id := g.groupedLightID(); id != "" {
			n.groupedLightIDs[g.ID] = id
		}
	}
}

// ListWithState lists groups with their aggregate grouped_light state and
// member lights filled in.
func (n *groupNamespace) ListWithState(ctx context.Context) ([]Group, error) {
	groups, err := n.List(ctx)
	if err != nil {
		return nil, err
	}

	var states APIResponse[GroupedLight]
	if err := n.client.get(ctx, "grouped_light", &states); err != nil {
		return nil, err
	}
	stateByID := make(map[string]*GroupedLight, len(states.Data))
	for i := range states.Data {
		stateByID[states.Data[i].ID] = &states.Data[i]
	}

	lights, err := n.client.Lights.List(ctx)
	if err != nil {
		return nil, err
	}

	for i := range groups {
		groups[i].State = stateByID[groups[i].groupedLightID()]
		for _, light := range lights {
			if groups[i].Type == ResourceTypeBridgeHome || groups[i].Contains(light) {
				groups[i].Lights = append(groups[i].Lights, light)
			}
		}
	}
	return groups, nil
}

// GetWithState returns one group by ID with state and member lights.
func (n *groupNamespace) GetWithState(ctx context.Context, id string) (Group, error) {
	groups, err := n.ListWithState(ctx)
	if err != nil {
		return Group{}, err
	}
	for _, g := range groups {
		if g.ID == id {
			return g, nil
		}
	}
	return Group{}, newResourceNotFoundError("no %s with ID %s", n.resourceType, id)
}

func (n *groupNamespace) groupedLightID(ctx context.Context, id string) (string, error) {
	n.mu.Lock()
	cached, ok := n.groupedLightIDs[id]
	n.mu.Unlock()
	if ok {
		return cached, nil
	}

	group, err := n.GetOne(ctx, id)
	if err != nil {
		return "", err
	}
	n.remember([]Group{group})
	if groupedLightID := group.groupedLightID(); groupedLightID != "" {
		return groupedLightID, nil
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
	return &Rooms{newGroupNamespace(client, ResourceTypeRoom)}
}

type Zones struct{ groupNamespace }

func newZones(client *Client) *Zones {
	return &Zones{newGroupNamespace(client, ResourceTypeZone)}
}
