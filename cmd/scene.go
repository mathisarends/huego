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
		scenes, err := client.Scenes.List(cmd.Context())
		if err != nil {
			return err
		}
		scope, err := sceneScope(cmd, client)
		if err != nil {
			return err
		}
		if scope != nil {
			filtered := scenes[:0]
			for _, scene := range scenes {
				if scene.Group == *scope {
					filtered = append(filtered, scene)
				}
			}
			scenes = filtered
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
		brightness, transition, err := brightnessAndTransitionFlags(cmd)
		if err != nil {
			return err
		}
		activation.Brightness, activation.Transition = brightness, transition
		activation.Dynamic, _ = cmd.Flags().GetBool("dynamic")

		client, err := controlClient(cmd, args[0])
		if err != nil {
			return err
		}
		id, err := resolveResourceTarget(cmd, client, "scene", args[0], false)
		if err != nil {
			return err
		}
		resp, err := client.Activate(cmd.Context(), id, activation)
		if err != nil {
			return err
		}
		return writeResponse(cmd, args[0], resp)
	},
}

func init() {
	addSceneScope(sceneCmd)
	sceneActivateCmd.Flags().Bool("dry-run", false, "Preview scene recall without writing")
	addBrightnessFlag(sceneActivateCmd)
	addTransitionFlag(sceneActivateCmd)
	sceneActivateCmd.Flags().Bool("dynamic", false, "Start the scene's dynamic palette")
	sceneCmd.AddCommand(sceneListCmd, sceneActivateCmd)
	rootCmd.AddCommand(sceneCmd)
}
