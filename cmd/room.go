package cmd

import (
	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

var roomCmd = &cobra.Command{
	Use:   "room",
	Short: "Inspect and control rooms",
}

var roomListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all rooms visible to the configured bridge",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		rooms, err := client.Rooms.List(cmd.Context())
		if err != nil {
			return err
		}
		return writeResourceRows(cmd, groupRows(rooms))
	},
}

func roomTarget(c *hue.Client) controlTarget { return c.Rooms }

func init() {
	roomCmd.AddCommand(roomListCmd)
	addControlCommands(roomCmd, roomTarget)
	rootCmd.AddCommand(roomCmd)
}
