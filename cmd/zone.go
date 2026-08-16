package cmd

import (
	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

var zoneCmd = &cobra.Command{
	Use:   "zone",
	Short: "Inspect and control zones",
}

var zoneListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all zones visible to the configured bridge",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		zones, err := client.Zones.ListWithState(cmd.Context())
		if err != nil {
			return err
		}
		return writeResourceRows(cmd, groupRows(zones))
	},
}

func zoneTarget(c *hue.Client) controlTarget { return c.Zones }

func init() {
	zoneCmd.AddCommand(zoneListCmd)
	addControlCommands(zoneCmd, zoneTarget)
	rootCmd.AddCommand(zoneCmd)
}
