package cmd

import (
	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

var homeCmd = &cobra.Command{
	Use:   "home",
	Short: "Inspect and control every light on the bridge at once",
	Long: "Inspect and control every light on the bridge at once through the bridge_home group.\n" +
		"Control commands take no target: \"huego home off\" switches off the whole home.",
}

var homeGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show the aggregate state of all lights",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		id, err := client.Home.ID(cmd.Context())
		if err != nil {
			return err
		}
		home, err := client.Home.GetWithState(cmd.Context(), id)
		if err != nil {
			return err
		}
		view := newGroupView(home)
		if view.Name == "" {
			view.Name = "Home"
		}
		return writeDetails(cmd, view, view.fields())
	},
}

func homeTarget(c *hue.Client) controlTarget { return c.Home }

func init() {
	homeCmd.AddCommand(homeGetCmd)
	addControlCommands(homeCmd, homeTarget, false)
	rootCmd.AddCommand(homeCmd)
}
