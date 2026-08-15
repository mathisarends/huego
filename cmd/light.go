package cmd

import (
	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

var lightCmd = &cobra.Command{
	Use:   "light",
	Short: "Inspect and control individual lights",
}

var lightListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all lights visible to the configured bridge",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		lights, err := client.Lights.List(cmd.Context())
		if err != nil {
			return err
		}
		return writeResourceRows(cmd, lightRows(lights))
	},
}

func lightTarget(c *hue.Client) controlTarget { return c.Lights }

func init() {
	lightCmd.AddCommand(lightListCmd)
	addControlCommands(lightCmd, lightTarget)
	rootCmd.AddCommand(lightCmd)
}
