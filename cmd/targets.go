package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

func addSceneScope(command *cobra.Command) {
	command.PersistentFlags().String("room", "", "Limit scenes to this room UUID or exact name")
	command.PersistentFlags().String("zone", "", "Limit scenes to this zone UUID or exact name")
	command.MarkFlagsMutuallyExclusive("room", "zone")
}

func hasSceneScope(cmd *cobra.Command) bool {
	room, _ := cmd.Flags().GetString("room")
	zone, _ := cmd.Flags().GetString("zone")
	return room != "" || zone != ""
}

func applySceneScope(cmd *cobra.Command, kind string, body json.RawMessage) (json.RawMessage, error) {
	if (kind != "scene" && kind != "smart_scene") || !hasSceneScope(cmd) {
		return body, nil
	}
	client, err := newClient()
	if err != nil {
		return nil, err
	}
	scope, err := sceneScope(cmd, client)
	if err != nil {
		return nil, err
	}
	var payload hue.Resource
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if group := decodeField[hue.ResourceReference](payload, "group"); group.RID != "" && group != *scope {
		return nil, fmt.Errorf("payload group does not match --room/--zone")
	}
	payload["group"] = jsonValue(scope)
	return jsonValue(payload), nil
}

func sceneScope(cmd *cobra.Command, client *hue.Client) (*hue.ResourceReference, error) {
	room, _ := cmd.Flags().GetString("room")
	zone, _ := cmd.Flags().GetString("zone")
	if room != "" && zone != "" {
		return nil, fmt.Errorf("choose --room or --zone")
	}
	kind, target := "room", room
	if zone != "" {
		kind, target = "zone", zone
	}
	if target == "" {
		return nil, nil
	}
	ns, err := client.Resources.Of(kind, false)
	if err != nil {
		return nil, err
	}
	id, err := ns.ResolveTarget(cmd.Context(), target)
	if err != nil {
		return nil, err
	}
	return &hue.ResourceReference{RID: id, RType: hue.ResourceType(kind)}, nil
}

func decodeField[T any](resource hue.Resource, key string) T {
	var value T
	_ = json.Unmarshal(resource[key], &value)
	return value
}

func readResources(cmd *cobra.Command, client *hue.Client, kind string) ([]hue.Resource, error) {
	raw, _ := cmd.Flags().GetBool("raw")
	ns, err := client.Resources.Of(kind, raw)
	if err != nil {
		return nil, err
	}
	response, err := ns.List(cmd.Context())
	if err != nil {
		return nil, err
	}
	if err := response.Err(); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func readResource(cmd *cobra.Command, client *hue.Client, kind, id string) (hue.Resource, error) {
	raw, _ := cmd.Flags().GetBool("raw")
	ns, err := client.Resources.Of(kind, raw)
	if err != nil {
		return nil, err
	}
	response, err := ns.Get(cmd.Context(), id)
	if err != nil {
		return nil, err
	}
	if err := response.Err(); err != nil {
		return nil, err
	}
	if len(response.Data) != 1 {
		return nil, &hue.Problem{Code: "target_not_found", Message: fmt.Sprintf("no %s with ID %s", kind, id)}
	}
	return response.Data[0], nil
}

func resolveResourceTarget(cmd *cobra.Command, client *hue.Client, kind, target string, raw bool) (string, error) {
	ns, err := client.Resources.Of(kind, raw)
	if err != nil {
		return "", err
	}
	if (kind != "scene" && kind != "smart_scene") || !hasSceneScope(cmd) {
		return ns.ResolveTarget(cmd.Context(), target)
	}
	scope, err := sceneScope(cmd, client)
	if err != nil {
		return "", err
	}
	resources, err := readResources(cmd, client, kind)
	if err != nil {
		return "", err
	}
	var candidates []hue.Resource
	for _, resource := range resources {
		if decodeField[hue.ResourceReference](resource, "group") == *scope {
			candidates = append(candidates, resource)
		}
	}
	return findTarget(candidates, kind, target)
}

func findTarget(resources []hue.Resource, kind, target string) (string, error) {
	var ids []string
	for _, resource := range resources {
		if (hue.IsUUID(target) && strings.EqualFold(resource.GetID(), target)) || (!hue.IsUUID(target) && strings.TrimSpace(target) != "" && strings.EqualFold(strings.TrimSpace(resource.GetName()), strings.TrimSpace(target))) {
			ids = append(ids, resource.GetID())
		}
	}
	if len(ids) == 1 {
		return ids[0], nil
	}
	if len(ids) > 1 {
		return "", &hue.Problem{Code: "ambiguous_target", Message: fmt.Sprintf("ambiguous %s name %q; use a UUID", kind, target), Details: map[string]any{"candidate_ids": ids}}
	}
	return "", &hue.Problem{Code: "target_not_found", Message: fmt.Sprintf("no %s matching %q in the selected scope", kind, target)}
}
