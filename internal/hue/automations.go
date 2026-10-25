package hue

import (
	"context"
	"encoding/json"
)

const (
	ResourceTypeBehaviorInstance ResourceType = "behavior_instance"
	ResourceTypeBehaviorScript   ResourceType = "behavior_script"
)

// Automation is a behavior_instance: one configured Hue automation, such as
// a wake-up, a timer, or a "coming home" routine.
type Automation struct {
	ID            string          `json:"id"`
	ScriptID      string          `json:"script_id"`
	Enabled       bool            `json:"enabled"`
	Status        string          `json:"status"`
	LastError     string          `json:"last_error,omitempty"`
	Metadata      NamedMetadata   `json:"metadata"`
	Configuration json.RawMessage `json:"configuration,omitempty"`
	ScriptName    string          `json:"-"`
}

func (a Automation) GetID() string   { return a.ID }
func (a Automation) GetName() string { return a.Metadata.Name }

type behaviorScript struct {
	ID       string        `json:"id"`
	Metadata NamedMetadata `json:"metadata"`
}

func (s behaviorScript) GetID() string   { return s.ID }
func (s behaviorScript) GetName() string { return s.Metadata.Name }

type Automations struct {
	resourceNamespace[Automation]
}

func newAutomations(client *Client) *Automations {
	return &Automations{resourceNamespace[Automation]{client: client, resourceType: string(ResourceTypeBehaviorInstance)}}
}

// ListWithScripts lists automations with ScriptName set to the kind of
// automation, such as "Wake up" or "Timers".
func (n *Automations) ListWithScripts(ctx context.Context) ([]Automation, error) {
	automations, err := n.List(ctx)
	if err != nil {
		return nil, err
	}
	scripts, err := resourceNamespace[behaviorScript]{client: n.client, resourceType: string(ResourceTypeBehaviorScript)}.List(ctx)
	if err != nil {
		return nil, err
	}
	names := make(map[string]string, len(scripts))
	for _, s := range scripts {
		names[s.ID] = s.GetName()
	}
	for i := range automations {
		automations[i].ScriptName = names[automations[i].ScriptID]
	}
	return automations, nil
}

func (n *Automations) SetEnabled(ctx context.Context, id string, enabled bool) (APIResponse[ResourceIdentifier], error) {
	var resp APIResponse[ResourceIdentifier]
	err := n.client.put(ctx, n.resourceType+"/"+id, map[string]bool{"enabled": enabled}, &resp)
	return resp, err
}
