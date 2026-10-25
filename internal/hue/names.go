package hue

import (
	"context"
	"encoding/json"
)

// HomeName labels the bridge_home group, which has no name of its own.
const HomeName = "Home"

// Names maps the ID of every resource on the bridge to a display name, read
// in a single request. Resources without a name of their own, such as a
// room's grouped_light or a device's sensors, take their owner's name.
// Scenes and smart scenes use their qualified "<group>/<scene>" name, since
// plain scene names repeat across rooms.
func (c *Client) Names(ctx context.Context) (map[string]string, error) {
	var resp APIResponse[json.RawMessage]
	if err := c.get(ctx, "", &resp); err != nil {
		return nil, err
	}
	type resource struct {
		ID       string             `json:"id"`
		Type     ResourceType       `json:"type"`
		Metadata NamedMetadata      `json:"metadata"`
		Owner    *ResourceReference `json:"owner"`
		Group    *ResourceReference `json:"group"`
	}
	resources := make([]resource, 0, len(resp.Data))
	for _, raw := range resp.Data {
		var r resource
		if err := json.Unmarshal(raw, &r); err != nil {
			return nil, err
		}
		resources = append(resources, r)
	}

	names := map[string]string{}
	for _, r := range resources {
		switch {
		case r.Type == ResourceTypeBridgeHome:
			names[r.ID] = HomeName
		case r.Metadata.Name != "":
			names[r.ID] = r.Metadata.Name
		}
	}
	for _, r := range resources {
		if r.Group == nil || (r.Type != ResourceTypeScene && r.Type != ResourceTypeSmartScene) {
			continue
		}
		if group, ok := names[r.Group.RID]; ok {
			names[r.ID] = group + "/" + r.Metadata.Name
		}
	}
	for _, r := range resources {
		if _, named := names[r.ID]; named || r.Owner == nil {
			continue
		}
		if owner, ok := names[r.Owner.RID]; ok {
			names[r.ID] = owner
		}
	}
	return names, nil
}
