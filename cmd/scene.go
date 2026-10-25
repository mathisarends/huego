package cmd

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

var sceneCmd = &cobra.Command{
	Use:     "scene",
	Aliases: []string{"scenes"},
	Short:   "Inspect and activate scenes",
}

var sceneListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List scenes with the room or zone they belong to",
	Example: "  huego scene list --group Office\n" +
		"  scene=$(huego scene list --names | walker --dmenu) && huego scene activate \"$scene\"",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		scenes, err := client.Scenes.ListWithGroupNames(cmd.Context())
		if err != nil {
			return err
		}
		if group, _ := cmd.Flags().GetString("group"); group != "" {
			if scenes, err = inGroup(scenes, group, "scenes", func(s hue.Scene) string { return s.GroupName }); err != nil {
				return err
			}
		}
		return writeList(cmd, mapViews(scenes, newSceneView), sceneListing)
	},
}

// inGroup keeps the items of the room or zone named group; kind names the
// items in the error when the group has none.
func inGroup[T any](items []T, group, kind string, groupOf func(T) string) ([]T, error) {
	var matched []T
	groups := map[string]bool{}
	for _, item := range items {
		groups[groupOf(item)] = true
		if strings.EqualFold(groupOf(item), strings.TrimSpace(group)) {
			matched = append(matched, item)
		}
	}
	if matched == nil {
		known := slices.Sorted(maps.Keys(groups))
		return nil, &hue.ResourceNotFoundError{Message: fmt.Sprintf("no room or zone named %q has %s. Known: %s", group, kind, strings.Join(known, ", "))}
	}
	return matched, nil
}

var sceneGetCmd = &cobra.Command{
	Use:   "get <target>",
	Short: "Show one scene",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		id, err := client.Scenes.ResolveTarget(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		scenes, err := client.Scenes.ListWithGroupNames(cmd.Context())
		if err != nil {
			return err
		}
		scene, err := findByID(scenes, id, "scene")
		if err != nil {
			return err
		}
		view := newSceneView(scene)
		return writeDetails(cmd, view, view.fields())
	},
}

var sceneActivateCmd = &cobra.Command{
	Use:   "activate <target>",
	Long:  "Activate a scene by UUID, name, or \"<room or zone>/<scene>\" when several rooms share the scene name.",
	Short: "Activate a scene, optionally with a brightness or fade",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		brightness, transition, err := brightnessAndTransitionFlags(cmd)
		if err != nil {
			return err
		}
		activation := hue.SceneActivation{Brightness: brightness, Transition: transition}
		activation.Dynamic, _ = cmd.Flags().GetBool("dynamic")

		client, err := newClient()
		if err != nil {
			return err
		}
		id, err := client.Scenes.ResolveTarget(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		resp, err := client.Scenes.Activate(cmd.Context(), id, activation)
		if err != nil {
			return err
		}
		return writeResponse(cmd, args[0], resp)
	},
}

func init() {
	addBrightnessFlag(sceneActivateCmd)
	addTransitionFlag(sceneActivateCmd)
	sceneActivateCmd.Flags().Bool("dynamic", false, "start the scene's dynamic palette")
	addNamesFlag(sceneListCmd)
	sceneListCmd.Flags().String("group", "", "only scenes of this room or zone")
	sceneCmd.AddCommand(sceneListCmd, sceneGetCmd, sceneActivateCmd)
	rootCmd.AddCommand(sceneCmd)
}
