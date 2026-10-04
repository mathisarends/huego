package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

func newNativeListCommand(kind string) *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "Read complete native resources", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newClient()
			if err != nil {
				return err
			}
			namespace, err := client.Resources.Of(kind, false)
			if err != nil {
				return err
			}
			response, err := namespace.List(cmd.Context())
			if err != nil {
				return err
			}
			return writeNativeResponse(cmd, response)
		},
	}
}

type payloadBuilder func([]string) (any, error)

var effectNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func newNativeAction(kind, use, short string, count int, build payloadBuilder) *cobra.Command {
	command := &cobra.Command{
		Use: use, Short: short, Args: cobra.ExactArgs(count),
		RunE: func(cmd *cobra.Command, args []string) error {
			payload, err := build(args[1:])
			if err != nil {
				return err
			}
			body, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			return runNativeWrite(cmd, kind, http.MethodPut, args[0], body)
		},
	}
	command.Flags().Bool("dry-run", false, "Print the request without writing; names may require a bridge read")
	return command
}

func fixedPayload(payload any) payloadBuilder {
	return func(_ []string) (any, error) { return payload, nil }
}

func schemaProperty(schema json.RawMessage, path ...string) bool {
	var object struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if len(path) == 0 {
		return len(schema) != 0
	}
	if json.Unmarshal(schema, &object) != nil {
		return false
	}
	return schemaProperty(object.Properties[path[0]], path[1:]...)
}

func addNativeHelpers(parent *cobra.Command, capability hue.ResourceCapability) {
	kind := capability.ResourceType
	schema := capability.RequestSchemas[http.MethodPut]
	if schemaProperty(schema, "enabled") {
		parent.AddCommand(
			newNativeAction(kind, "enable <target>", "Enable this sensor or automation", 1, fixedPayload(map[string]any{"enabled": true})),
			newNativeAction(kind, "disable <target>", "Disable this sensor or automation", 1, fixedPayload(map[string]any{"enabled": false})),
		)
	}
	metadataName := schemaProperty(schema, "metadata", "name")
	if metadataName || schemaProperty(schema, "name") {
		parent.AddCommand(newNativeAction(kind, "rename <target> <name>", "Rename a resource", 2, func(args []string) (any, error) {
			name := strings.TrimSpace(args[0])
			if name == "" || utf8.RuneCountInString(name) > 32 {
				return nil, fmt.Errorf("name must contain 1 to 32 characters")
			}
			if metadataName {
				return map[string]any{"metadata": map[string]string{"name": name}}, nil
			}
			return map[string]string{"name": name}, nil
		}))
	}
	switch kind {
	case "motion":
		parent.AddCommand(newNativeAction(kind, "sensitivity <target> <value>", "Set motion sensitivity; the bridge enforces sensitivity_max", 2, func(args []string) (any, error) {
			value, err := parseInt(args[0], "sensitivity")
			if err != nil {
				return nil, err
			}
			if value < 0 {
				return nil, fmt.Errorf("sensitivity must be non-negative")
			}
			return map[string]any{"sensitivity": map[string]int{"sensitivity": value}}, nil
		}))
	case "device":
		parent.AddCommand(
			newNativeAction(kind, "identify <target>", "Trigger device identification", 1, fixedPayload(map[string]any{"identify": map[string]string{"action": "identify"}})),
			newNativeAction(kind, "usertest <target> <true|false>", "Enable or disable device usertest mode", 2, boolPayload("usertest", "usertest")),
		)
	case "smart_scene":
		for _, action := range []string{"activate", "deactivate"} {
			parent.AddCommand(newNativeAction(kind, action+" <target>", action+" a smart scene schedule", 1,
				fixedPayload(map[string]any{"recall": map[string]string{"action": action}})))
		}
	case "entertainment_configuration":
		for _, action := range []string{"start", "stop"} {
			parent.AddCommand(newNativeAction(kind, action+" <target>", action+" an entertainment area", 1, fixedPayload(map[string]string{"action": action})))
		}
	case "zigbee_device_discovery":
		parent.AddCommand(newNativeAction(kind, "search <target>", "Start Zigbee device discovery", 1,
			fixedPayload(map[string]any{"action": map[string]string{"action_type": "search"}})))
	case "device_software_update":
		parent.AddCommand(newNativeAction(kind, "install <target>", "Request installation of an available device update", 1,
			fixedPayload(map[string]any{"install": map[string]string{"install_state": "install"}})))
	case "geofence_client":
		parent.AddCommand(newNativeAction(kind, "at-home <target> <true|false>", "Set whether a geofence client is at home", 2, boolPayload("is_at_home")))
	case "light":
		parent.AddCommand(
			newNativeAction(kind, "effect <target> <effect>", "Set an effects_v2 effect, or no_effect to stop; requires light support", 2, func(args []string) (any, error) {
				if !effectNamePattern.MatchString(args[0]) {
					return nil, fmt.Errorf("effect must be a CLIP effect name, such as candle or no_effect")
				}
				return map[string]any{"effects_v2": map[string]any{"action": map[string]string{"effect": args[0]}}}, nil
			}),
			newNativeAction(kind, "powerup <target> <preset>", "Set safety, powerfail or last_on_state power-up behavior", 2, func(args []string) (any, error) {
				if !slices.Contains([]string{"safety", "powerfail", "last_on_state"}, args[0]) {
					return nil, fmt.Errorf("preset must be safety, powerfail or last_on_state; custom settings use light update")
				}
				return map[string]any{"powerup": map[string]string{"preset": args[0]}}, nil
			}),
		)
	}
}

func boolPayload(path ...string) payloadBuilder {
	return func(args []string) (any, error) {
		if args[0] != "true" && args[0] != "false" {
			return nil, fmt.Errorf("value must be true or false")
		}
		var value any = args[0] == "true"
		for i := len(path) - 1; i >= 0; i-- {
			value = map[string]any{path[i]: value}
		}
		return value, nil
	}
}

func init() {
	existing := map[string]*cobra.Command{"light": lightCmd, "room": roomCmd, "zone": zoneCmd, "scene": sceneCmd}
	for _, capability := range hue.Catalog().Resources {
		kind := capability.ResourceType
		parent := existing[kind]
		if parent == nil {
			name := strings.ReplaceAll(kind, "_", "-")
			parent = &cobra.Command{Use: name, Short: "Inspect and configure " + kind + " resources"}
			if name != kind {
				parent.Aliases = []string{kind}
			}
			parent.AddCommand(newNativeListCommand(kind))
			rootCmd.AddCommand(parent)
		}
		parent.Args = cobra.NoArgs
		parent.RunE = func(cmd *cobra.Command, args []string) error { return cmd.Help() }
		parent.AddCommand(newNativeGetCommand(kind))
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
			if capability.Supports(method, method != http.MethodPost) {
				parent.AddCommand(newNativeWriteCommand(kind, method))
			}
		}
		addNativeHelpers(parent, capability)
	}
}
