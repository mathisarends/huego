package cmd

import (
	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

var lightCmd = &cobra.Command{
	Use:     "light",
	Aliases: []string{"lights"},
	Short:   "Inspect and control individual lights",
}

var lightListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List lights with their room, state, brightness, and color",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		lights, err := client.Lights.ListWithRooms(cmd.Context())
		if err != nil {
			return err
		}
		return writeList(cmd, mapViews(lights, newLightView), lightListing)
	},
}

var lightGetCmd = &cobra.Command{
	Use:   "get <target>",
	Short: "Show one light's state and capabilities",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, id, err := resolveTarget(cmd, lightTarget, args[0])
		if err != nil {
			return err
		}
		lights, err := client.Lights.ListWithRooms(cmd.Context())
		if err != nil {
			return err
		}
		light, err := findByID(lights, id, "light")
		if err != nil {
			return err
		}
		view := newLightView(light)
		return writeDetails(cmd, view, view.fields())
	},
}

func lightTarget(c *hue.Client) controlTarget { return c.Lights }

func init() {
	addNamesFlag(lightListCmd)
	lightCmd.AddCommand(lightListCmd, lightGetCmd)
	addControlCommands(lightCmd, lightTarget)
	rootCmd.AddCommand(lightCmd)
}
