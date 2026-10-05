package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

func checkRequest(cmd *cobra.Command, client *hue.Client, kind, id string, body json.RawMessage) error {
	response, err := client.Resources.List(cmd.Context())
	if err != nil {
		return err
	}
	if err := response.Err(); err != nil {
		return err
	}
	index := map[string]hue.Resource{}
	lights := []hue.Resource{}
	for _, resource := range response.Data {
		resourceKind := decodeField[string](resource, "type")
		index[resourceKind+"/"+resource.GetID()] = resource
		if resourceKind == "light" {
			lights = append(lights, resource)
		}
	}
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return err
	}
	var references func(any) error
	references = func(value any) error {
		switch node := value.(type) {
		case map[string]any:
			rid, hasID := node["rid"].(string)
			rtype, hasType := node["rtype"].(string)
			if hasID && hasType && rtype != "public_image" {
				if _, ok := index[rtype+"/"+rid]; !ok {
					return &hue.Problem{Code: "reference_not_found", Message: fmt.Sprintf("referenced %s %s is absent from the bridge", rtype, rid)}
				}
			}
			for _, child := range node {
				if err := references(child); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range node {
				if err := references(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := references(value); err != nil {
		return err
	}
	var payload hue.Resource
	_ = json.Unmarshal(body, &payload)
	if kind == "light" {
		light, ok := index["light/"+id]
		if !ok {
			return &hue.Problem{Code: "target_not_found", Message: "light is absent from bridge"}
		}
		return checkLightCapabilities(light, payload)
	}
	if kind == "behavior_instance" {
		scriptID := decodeField[string](payload, "script_id")
		if scriptID == "" {
			scriptID = decodeField[string](index[kind+"/"+id], "script_id")
		}
		script, ok := index["behavior_script/"+scriptID]
		if !ok {
			return &hue.Problem{Code: "reference_not_found", Message: "behavior script is absent from the bridge"}
		}
		for field, schemaField := range map[string]string{"configuration": "configuration_schema", "trigger": "trigger_schema"} {
			if body, ok := payload[field]; ok {
				schema := script[schemaField]
				if len(schema) == 0 {
					return &hue.Problem{Code: "schema_unavailable", Message: "script does not expose " + schemaField}
				}
				if err := hue.ValidateSchema(schema, body); err != nil {
					return err
				}
			}
		}
	}
	if kind == "scene" || kind == "smart_scene" {
		group := decodeField[hue.ResourceReference](payload, "group")
		if group.RID == "" {
			group = decodeField[hue.ResourceReference](index[kind+"/"+id], "group")
		}
		groupResource, ok := index[string(group.RType)+"/"+group.RID]
		if !ok {
			return &hue.Problem{Code: "reference_not_found", Message: "scene group is absent from bridge"}
		}
		if kind == "scene" {
			members := groupLightIDs(groupResource, lights)
			for _, action := range decodeField[[]hue.Resource](payload, "actions") {
				target := decodeField[hue.ResourceReference](action, "target")
				if target.RType != "light" || !members[target.RID] {
					return fmt.Errorf("scene action target %s is not a light in the selected group", target.RID)
				}
				if err := checkLightCapabilities(index["light/"+target.RID], decodeField[hue.Resource](action, "action")); err != nil {
					return err
				}
			}
		} else {
			for _, day := range decodeField[[]hue.Resource](payload, "week_timeslots") {
				for _, slot := range decodeField[[]hue.Resource](day, "timeslots") {
					target := decodeField[hue.ResourceReference](slot, "target")
					if target.RType != "scene" || decodeField[hue.ResourceReference](index["scene/"+target.RID], "group") != group {
						return fmt.Errorf("smart-scene timeslot must target a scene in the same room or zone")
					}
				}
			}
		}
	}
	return nil
}

func init() {
	rootCmd.PersistentFlags().Bool("check", false, "Read bridge inventory to check write references, scene membership and light/script capabilities")
}
