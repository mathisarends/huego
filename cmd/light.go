package cmd

import (
	"fmt"
	"slices"
	"strings"

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

var lightEffectCmd = &cobra.Command{
	Use:   "effect <target> <effect>",
	Short: "Start a light effect such as candle or fire; \"none\" stops it",
	Long: "Start a light effect such as candle, fire, sparkle, or prism; \"none\" stops it.\n" +
		"Which effects a light supports is listed under capabilities in \"huego light get\".",
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, id, err := resolveTarget(cmd, lightTarget, args[0])
		if err != nil {
			return err
		}
		effect := strings.ToLower(args[1])
		if effect == "none" || effect == "off" {
			effect = hue.EffectNone
		}
		light, err := client.Lights.GetOne(cmd.Context(), id)
		if err != nil {
			return err
		}
		supported := light.SupportedEffects()
		if len(supported) == 0 {
			return fmt.Errorf("light %q does not support effects", light.Name())
		}
		if effect != hue.EffectNone && !slices.Contains(supported, effect) {
			return fmt.Errorf("light %q does not support effect %q; supported: %s, none", light.Name(), args[1], strings.Join(supported, ", "))
		}
		resp, err := client.Lights.SetEffect(cmd.Context(), id, effect)
		if err != nil {
			return err
		}
		return writeResponse(cmd, args[0], resp)
	},
}

func lightTarget(c *hue.Client) controlTarget { return c.Lights }

func init() {
	addNamesFlag(lightListCmd)
	lightCmd.AddCommand(lightListCmd, lightGetCmd, lightEffectCmd)
	addControlCommands(lightCmd, lightTarget, true)
	rootCmd.AddCommand(lightCmd)
}
