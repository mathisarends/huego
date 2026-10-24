package cmd

import (
	"fmt"

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
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		scenes, err := client.Scenes.ListWithGroupNames(cmd.Context())
		if err != nil {
			return err
		}
		return writeList(cmd, mapViews(scenes, newSceneView), sceneListing)
	},
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
		var activation hue.SceneActivation
		if cmd.Flags().Changed("brightness") {
			v, err := cmd.Flags().GetFloat64("brightness")
			if err != nil {
				return err
			}
			if v < 0 || v > 100 {
				return fmt.Errorf("brightness must be between 0 and 100")
			}
			activation.Brightness = &v
		}
		if cmd.Flags().Changed("transition") {
			v, err := cmd.Flags().GetFloat64("transition")
			if err != nil {
				return err
			}
			activation.Transition = &v
		}
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
	sceneActivateCmd.Flags().Bool("dynamic", false, "Start the scene's dynamic palette")
	addNamesFlag(sceneListCmd)
	sceneCmd.AddCommand(sceneListCmd, sceneGetCmd, sceneActivateCmd)
	rootCmd.AddCommand(sceneCmd)
}
