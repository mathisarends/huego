package cmd

import (
	"context"
	"slices"
	"strings"

	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

type nameLister func(ctx context.Context, c *hue.Client) ([]string, error)

func namesOf[T any](items []T, err error, name func(T) string) ([]string, error) {
	if err != nil {
		return nil, err
	}
	names := make([]string, len(items))
	for i, item := range items {
		names[i] = name(item)
	}
	return names, nil
}

// targetListers complete the <target> or <device> argument of each command
// group's subcommands.
var targetListers = map[string]nameLister{
	"light": func(ctx context.Context, c *hue.Client) ([]string, error) {
		lights, err := c.Lights.List(ctx)
		return namesOf(lights, err, hue.Light.Name)
	},
	"room": func(ctx context.Context, c *hue.Client) ([]string, error) {
		rooms, err := c.Rooms.List(ctx)
		return namesOf(rooms, err, hue.Group.Name)
	},
	"zone": func(ctx context.Context, c *hue.Client) ([]string, error) {
		zones, err := c.Zones.List(ctx)
		return namesOf(zones, err, hue.Group.Name)
	},
	"scene": func(ctx context.Context, c *hue.Client) ([]string, error) {
		scenes, err := c.Scenes.ListWithGroupNames(ctx)
		return namesOf(scenes, err, hue.Scene.QualifiedName)
	},
	"smart-scene": func(ctx context.Context, c *hue.Client) ([]string, error) {
		scenes, err := c.SmartScenes.ListWithGroupNames(ctx)
		return namesOf(scenes, err, hue.SmartScene.QualifiedName)
	},
	"automation": func(ctx context.Context, c *hue.Client) ([]string, error) {
		automations, err := c.Automations.List(ctx)
		return namesOf(automations, err, hue.Automation.GetName)
	},
	"device": func(ctx context.Context, c *hue.Client) ([]string, error) {
		devices, err := c.Devices.List(ctx)
		return namesOf(devices, err, hue.Device.GetName)
	},
	"sensor": func(ctx context.Context, c *hue.Client) ([]string, error) {
		devices, err := c.Devices.ListDetailed(ctx)
		if err != nil {
			return nil, err
		}
		var names []string
		for _, device := range devices {
			if len(device.Sensors) > 0 {
				names = append(names, device.GetName())
			}
		}
		return names, nil
	},
}

var groupFlagListers = map[string]nameLister{
	"room": targetListers["room"],
	"zone": targetListers["zone"],
	"group": func(ctx context.Context, c *hue.Client) ([]string, error) {
		rooms, err := targetListers["room"](ctx, c)
		if err != nil {
			return nil, err
		}
		zones, err := targetListers["zone"](ctx, c)
		return append(rooms, zones...), err
	},
}

var apiPaths = []string{
	"", "light", "grouped_light", "room", "zone", "bridge_home", "scene", "smart_scene", "device",
	"device_power", "zigbee_connectivity", "motion", "temperature", "light_level", "contact",
	"button", "relative_rotary", "entertainment_configuration", "entertainment", "behavior_instance",
	"behavior_script", "geofence_client", "geolocation", "homekit", "matter", "bridge",
}

// installCompletions gives every leaf command completion for its positional
// arguments, read from the <placeholders> in its Use line, and turns off
// file-name completion, which no huego argument takes.
func installCompletions(cmd *cobra.Command, targets nameLister) {
	if lister, ok := targetListers[cmd.Name()]; ok && cmd.Parent() == rootCmd {
		targets = lister
	}
	if !cmd.HasSubCommands() && cmd.ValidArgsFunction == nil {
		cmd.ValidArgsFunction = positionalCompletion(targets)
	}
	for flag, lister := range groupFlagListers {
		if cmd.LocalFlags().Lookup(flag) != nil {
			_ = cmd.RegisterFlagCompletionFunc(flag, func(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
				return bridgeCompletions(cmd, lister)
			})
		}
	}
	for _, sub := range cmd.Commands() {
		installCompletions(sub, targets)
	}
}

func positionalCompletion(targets nameLister) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		placeholders := strings.Fields(cmd.Use)[1:]
		if len(args) >= len(placeholders) {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		switch strings.Trim(placeholders[len(args)], "[]") {
		case "<target>", "<device>":
			return bridgeCompletions(cmd, targets)
		case "<color>":
			return colorNames(), cobra.ShellCompDirectiveNoFileComp
		case "<effect>":
			return lightEffects(cmd, args[0])
		case "<method>":
			return []string{"get", "put", "post", "delete"}, cobra.ShellCompDirectiveNoFileComp
		case "<path>":
			return apiPaths, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func bridgeCompletions(cmd *cobra.Command, list nameLister) ([]string, cobra.ShellCompDirective) {
	if list == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	client, err := newClient()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	names, err := list(cmd.Context(), client)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	slices.Sort(names)
	return slices.Compact(names), cobra.ShellCompDirectiveNoFileComp
}

func lightEffects(cmd *cobra.Command, target string) ([]string, cobra.ShellCompDirective) {
	client, err := newClient()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	id, err := client.Lights.ResolveTarget(cmd.Context(), target)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	light, err := client.Lights.GetOne(cmd.Context(), id)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	return append([]string{"none"}, light.SupportedEffects()...), cobra.ShellCompDirectiveNoFileComp
}
