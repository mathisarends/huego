package hue

import "context"

type LightNamespace struct {
	resourceNamespace[Light]
}

func newLightNamespace(client *Client) *LightNamespace {
	return &LightNamespace{resourceNamespace[Light]{client: client, resourceType: "light"}}
}

func (n *LightNamespace) Apply(ctx context.Context, id string, update LightUpdate) (APIResponse[ResourceIdentifier], error) {
	var resp APIResponse[ResourceIdentifier]
	err := n.client.put(ctx, "light/"+id, update, &resp)
	return resp, err
}

func (n *LightNamespace) IsOn(ctx context.Context, id string) (bool, error) {
	light, err := n.GetOne(ctx, id)
	if err != nil {
		return false, err
	}
	return light.On.On, nil
}
