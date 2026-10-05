package cmd

import (
	"encoding/json"

	"github.com/mathisarends/huego/internal/config"
	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.PersistentFlags().String("bridge", "", "Use a named bridge profile (isolates credentials from environment overrides)")
	parent := &cobra.Command{Use: "profile", Short: "Discover bridges and manage named connection profiles"}
	parent.AddCommand(&cobra.Command{Use: "list", Short: "List saved profiles without credentials", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		profiles, err := config.Profiles()
		if err != nil {
			return err
		}
		return writeStructured(cmd, profiles)
	}})
	parent.AddCommand(&cobra.Command{Use: "discover", Short: "Discover bridge addresses without pairing", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		bridges, err := hue.DiscoverBridges(cmd.Context())
		if err != nil {
			return err
		}
		return writeStructured(cmd, bridges)
	}})
	save := &cobra.Command{Use: "save <name>", Short: "Save or replace a profile from bridge_ip/app_key JSON", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		body, err := readPayload(cmd)
		if err != nil {
			return err
		}
		var bridge config.Bridge
		if err = json.Unmarshal(body, &bridge); err != nil {
			return err
		}
		if err = config.SaveProfile(args[0], bridge); err != nil {
			return err
		}
		return writeStructured(cmd, config.Profile{Name: args[0], IP: bridge.IP})
	}}
	addPayloadFlags(save)
	parent.AddCommand(save)
	rootCmd.AddCommand(parent)
}
