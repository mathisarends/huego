package cmd

import (
	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

var sceneCmd = &cobra.Command{
	Use:   "scene",
	Short: "Inspect and activate scenes",
}

var sceneListCmd = &cobra.Command{
	Use:   "list",
	Short: "List scenes visible to the configured bridge",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		scenes, err := client.SceneNamespace.List(cmd.Context())
		if err != nil {
			return err
		}
		return writeResourceRows(cmd, sceneRows(scenes))
	},
}

var sceneActivateCmd = &cobra.Command{
	Use:   "activate <target>",
	Short: "Activate a scene, optionally with a brightness or fade",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var activation hue.SceneActivation
		if cmd.Flags().Changed("brightness") {
			v, err := cmd.Flags().GetFloat64("brightness")
			if err != nil {
				return err
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
		id, err := client.SceneNamespace.ResolveTarget(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		resp, err := client.SceneNamespace.Activate(cmd.Context(), id, activation)
		if err != nil {
			return err
		}
		return writeResponse(cmd, resp)
	},
}

func init() {
	addBrightnessFlag(sceneActivateCmd)
	addTransitionFlag(sceneActivateCmd)
	sceneActivateCmd.Flags().Bool("dynamic", false, "Start the scene's dynamic palette")
	sceneCmd.AddCommand(sceneListCmd, sceneActivateCmd)
	rootCmd.AddCommand(sceneCmd)
}
