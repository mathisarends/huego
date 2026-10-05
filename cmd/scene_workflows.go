package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

func sceneSnapshotCommand() *cobra.Command {
	command := &cobra.Command{Use: "snapshot <name>", Short: "Save the current lights of --room or --zone as a scene", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		scope, err := sceneScope(cmd, client)
		if err != nil {
			return err
		}
		if scope == nil {
			return fmt.Errorf("snapshot requires --room or --zone")
		}
		group, err := readResource(cmd, client, string(scope.RType), scope.RID)
		if err != nil {
			return err
		}
		lights, err := readResources(cmd, client, "light")
		if err != nil {
			return err
		}
		members := groupLightIDs(group, lights)
		actions := []hue.Resource{}
		for _, light := range lights {
			if members[light.GetID()] {
				action := hue.Resource{}
				for _, key := range []string{"on", "dimming"} {
					if value, ok := light[key]; ok {
						var state hue.Resource
						_ = json.Unmarshal(value, &state)
						if key == "dimming" {
							delete(state, "min_dim_level")
						}
						action[key] = jsonValue(state)
					}
				}
				temperature := decodeField[hue.Resource](light, "color_temperature")
				if decodeField[bool](temperature, "mirek_valid") && string(temperature["mirek"]) != "null" && len(temperature["mirek"]) > 0 {
					action["color_temperature"] = jsonValue(hue.Resource{"mirek": temperature["mirek"]})
				} else if color := decodeField[hue.Resource](light, "color"); len(color["xy"]) > 0 {
					action["color"] = jsonValue(hue.Resource{"xy": color["xy"]})
				}
				if gradient := decodeField[hue.Resource](light, "gradient"); len(gradient["points"]) > 0 {
					copy := hue.Resource{"points": gradient["points"]}
					if len(gradient["mode"]) > 0 {
						copy["mode"] = gradient["mode"]
					}
					action["gradient"] = jsonValue(copy)
				}
				if effects := decodeField[hue.Resource](light, "effects"); len(effects["status"]) > 0 {
					action["effects"] = jsonValue(hue.Resource{"effect": effects["status"]})
				}
				actions = append(actions, hue.Resource{"target": jsonValue(hue.ResourceReference{RID: light.GetID(), RType: "light"}), "action": jsonValue(action)})
			}
		}
		if len(actions) == 0 {
			return fmt.Errorf("selected group contains no lights")
		}
		payload := hue.Resource{"metadata": jsonValue(map[string]string{"name": args[0]}), "group": jsonValue(scope), "actions": jsonValue(actions)}
		ensure, _ := cmd.Flags().GetBool("ensure")
		if ensure {
			return ensureResource(cmd, client, "scene", args[0], payload)
		}
		return executeChanges(cmd, client, []resourceChange{{Kind: "scene", Method: http.MethodPost, Body: payload}})
	}}
	command.Flags().Bool("ensure", false, "Update a scene with the same name and group instead of creating duplicates")
	command.Flags().Bool("dry-run", false, "Read current light states and preview the scene")
	return command
}

func sceneCloneCommand() *cobra.Command {
	command := &cobra.Command{Use: "clone <source> <name>", Short: "Copy a scene's writable settings in the same room or zone", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		id, err := resolveResourceTarget(cmd, client, "scene", args[0], false)
		if err != nil {
			return err
		}
		source, err := readResource(cmd, client, "scene", id)
		if err != nil {
			return err
		}
		payload := hue.Resource{}
		for _, key := range []string{"group", "actions", "palette", "speed", "auto_dynamic"} {
			if value, ok := source[key]; ok {
				payload[key] = value
			}
		}
		metadata := decodeField[hue.Resource](source, "metadata")
		if metadata == nil {
			metadata = hue.Resource{}
		}
		metadata["name"] = jsonValue(args[1])
		payload["metadata"] = jsonValue(metadata)
		ensure, _ := cmd.Flags().GetBool("ensure")
		if ensure {
			return ensureResource(cmd, client, "scene", args[1], payload)
		}
		return executeChanges(cmd, client, []resourceChange{{Kind: "scene", Method: http.MethodPost, Body: payload}})
	}}
	command.Flags().Bool("ensure", false, "Update an existing copy by name and group")
	command.Flags().Bool("dry-run", false, "Preview the copied scene")
	return command
}

func sceneLightCommand(remove bool) *cobra.Command {
	use, short := "set-light <scene> <light>", "Change one scene action while preserving other lights and fields"
	if remove {
		use, short = "remove-light <scene> <light>", "Remove one scene action while preserving the other lights"
	}
	command := &cobra.Command{Use: use, Short: short, Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		var action hue.Resource
		var err error
		if !remove {
			action, err = sceneActionPayload(cmd)
			if err != nil {
				return err
			}
		}
		client, err := newClient()
		if err != nil {
			return err
		}
		id, err := resolveResourceTarget(cmd, client, "scene", args[0], false)
		if err != nil {
			return err
		}
		scene, err := readResource(cmd, client, "scene", id)
		if err != nil {
			return err
		}
		lightID, err := resolveResourceTarget(cmd, client, "light", args[1], false)
		if err != nil {
			return err
		}
		if !remove {
			scope := decodeField[hue.ResourceReference](scene, "group")
			group, err := readResource(cmd, client, string(scope.RType), scope.RID)
			if err != nil {
				return err
			}
			light, err := readResource(cmd, client, "light", lightID)
			if err != nil {
				return err
			}
			if !groupLightIDs(group, []hue.Resource{light})[lightID] {
				return fmt.Errorf("light is not a member of the scene's room or zone")
			}
			if err := checkLightCapabilities(light, action); err != nil {
				return err
			}
		}
		actions := decodeField[[]hue.Resource](scene, "actions")
		next := make([]hue.Resource, 0, len(actions)+1)
		found := false
		for _, entry := range actions {
			if decodeField[hue.ResourceReference](entry, "target").RID == lightID {
				found = true
				if remove {
					continue
				}
				existing := decodeField[hue.Resource](entry, "action")
				entry["action"] = jsonValue(mergeObjects(existing, action))
			}
			next = append(next, entry)
		}
		if !remove && !found {
			next = append(next, hue.Resource{"target": jsonValue(hue.ResourceReference{RID: lightID, RType: "light"}), "action": jsonValue(action)})
		}
		changes := []resourceChange{}
		if !sameJSON(scene["actions"], jsonValue(next)) {
			changes = append(changes, resourceChange{Kind: "scene", ID: id, Method: http.MethodPut, Body: hue.Resource{"actions": jsonValue(next)}, GuardField: "actions", Before: scene["actions"]})
		}
		return executeChanges(cmd, client, changes)
	}}
	command.Flags().Bool("dry-run", false, "Read the scene and preview the changed action")
	if !remove {
		command.Flags().String("data", "", "Native action JSON (not a complete scene)")
		command.Flags().String("file", "", "Read native action JSON from a file or - for stdin")
		command.MarkFlagsMutuallyExclusive("data", "file")
		command.Flags().Bool("on", false, "Turn this light on when the scene is recalled")
		command.Flags().Bool("off", false, "Turn this light off when the scene is recalled")
		command.MarkFlagsMutuallyExclusive("on", "off")
		addBrightnessFlag(command)
		addTransitionFlag(command)
		command.Flags().String("color", "", "Color name or hex value")
		command.Flags().Int("temperature", 0, "Color temperature in kelvin")
		command.MarkFlagsMutuallyExclusive("color", "temperature")
	}
	return command
}

func sceneActionPayload(cmd *cobra.Command) (hue.Resource, error) {
	if cmd.Flags().Changed("data") || cmd.Flags().Changed("file") {
		for _, flag := range []string{"on", "off", "brightness", "transition", "color", "temperature"} {
			if cmd.Flags().Changed(flag) {
				return nil, fmt.Errorf("use action JSON or lighting flags, not both")
			}
		}
		body, err := readPayload(cmd)
		if err != nil {
			return nil, err
		}
		var action hue.Resource
		err = json.Unmarshal(body, &action)
		return action, err
	}
	brightness, transition, err := brightnessAndTransitionFlags(cmd)
	if err != nil {
		return nil, err
	}
	change := hue.StateChange{Brightness: brightness, Transition: transition}
	if cmd.Flags().Changed("on") {
		v, _ := cmd.Flags().GetBool("on")
		change.On = &v
	}
	if cmd.Flags().Changed("off") {
		v, _ := cmd.Flags().GetBool("off")
		v = !v
		change.On = &v
	}
	if cmd.Flags().Changed("color") {
		value, _ := cmd.Flags().GetString("color")
		rgb, err := hue.ParseColor(value)
		if err != nil {
			return nil, err
		}
		xy := hue.RGBToXY(rgb)
		change.Color = &xy
	}
	if cmd.Flags().Changed("temperature") {
		value, _ := cmd.Flags().GetInt("temperature")
		change.Kelvin = &value
	}
	update, err := hue.BuildLightUpdate(change)
	if err != nil {
		return nil, err
	}
	var action hue.Resource
	_ = json.Unmarshal(jsonValue(update), &action)
	if len(action) == 0 {
		return nil, fmt.Errorf("provide action JSON or at least one lighting flag")
	}
	return action, nil
}

func mergeObjects(current, patch hue.Resource) hue.Resource {
	if current == nil {
		current = hue.Resource{}
	}
	for key, value := range patch {
		var nested hue.Resource
		if json.Unmarshal(value, &nested) == nil && nested != nil {
			current[key] = jsonValue(mergeObjects(decodeField[hue.Resource](current, key), nested))
		} else {
			current[key] = value
		}
	}
	if _, ok := patch["color"]; ok {
		delete(current, "color_temperature")
	}
	if _, ok := patch["color_temperature"]; ok {
		delete(current, "color")
	}
	return current
}

func checkLightCapabilities(light, action hue.Resource) error {
	for _, field := range []string{"dimming", "color", "color_temperature", "gradient", "effects", "effects_v2"} {
		if _, ok := action[field]; ok {
			if _, supported := light[field]; !supported {
				return &hue.Problem{Code: "unsupported_capability", Message: fmt.Sprintf("light %s does not support %s", light.GetID(), field)}
			}
		}
	}
	if temp := decodeField[hue.Resource](action, "color_temperature"); len(temp["mirek"]) > 0 && string(temp["mirek"]) != "null" {
		mirek := decodeField[int](temp, "mirek")
		schema := decodeField[hue.Resource](decodeField[hue.Resource](light, "color_temperature"), "mirek_schema")
		min, max := decodeField[int](schema, "mirek_minimum"), decodeField[int](schema, "mirek_maximum")
		if max > 0 && (mirek < min || mirek > max) {
			return fmt.Errorf("mirek must be within this light's range %d-%d", min, max)
		}
	}
	return nil
}

func init() {
	sceneCmd.AddCommand(sceneSnapshotCommand(), sceneCloneCommand(), sceneLightCommand(false), sceneLightCommand(true))
}
