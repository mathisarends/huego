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
