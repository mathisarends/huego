package cmd

import (
	"context"

	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

// controlTarget is satisfied by *hue.LightNamespace and by
// *hue.RoomNamespace/*hue.ZoneNamespace, which act through their
// grouped_light service instead of light-by-light.
type controlTarget interface {
	hue.Controllable
	ResolveTarget(ctx context.Context, target string) (string, error)
}

// addControlCommands attaches the on/off/toggle/brightness/color/temperature/
// identify vocabulary shared by lights, rooms, and zones to a parent command.
func addControlCommands(parent *cobra.Command, target func(*hue.Client) controlTarget) {
	onCmd := &cobra.Command{
		Use:   "on <target>",
		Short: "Turn a light, room, or zone on",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			brightness, transition, err := brightnessAndTransitionFlags(cmd)
			if err != nil {
				return err
			}
			client, id, err := resolveTarget(cmd, target, args[0])
			if err != nil {
				return err
			}
			resp, err := hue.TurnOn(cmd.Context(), target(client), id, brightness, nil, transition)
			if err != nil {
				return err
			}
			return writeResponse(cmd, resp)
		},
	}
	addBrightnessFlag(onCmd)
	addTransitionFlag(onCmd)

	offCmd := &cobra.Command{
		Use:   "off <target>",
		Short: "Turn a light, room, or zone off",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, transition, err := brightnessAndTransitionFlags(cmd)
			if err != nil {
				return err
			}
			client, id, err := resolveTarget(cmd, target, args[0])
			if err != nil {
				return err
			}
			resp, err := hue.TurnOff(cmd.Context(), target(client), id, transition)
			if err != nil {
				return err
			}
			return writeResponse(cmd, resp)
		},
	}
	addTransitionFlag(offCmd)

	toggleCmd := &cobra.Command{
		Use:   "toggle <target>",
		Short: "Toggle a light, room, or zone",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, transition, err := brightnessAndTransitionFlags(cmd)
			if err != nil {
				return err
			}
			client, id, err := resolveTarget(cmd, target, args[0])
			if err != nil {
				return err
			}
			resp, err := hue.Toggle(cmd.Context(), target(client), id, transition)
			if err != nil {
				return err
			}
			return writeResponse(cmd, resp)
		},
	}
	addTransitionFlag(toggleCmd)

	brightnessCmd := &cobra.Command{
		Use:   "brightness <target> <percent>",
		Short: "Set brightness from 0 to 100; zero turns the target off",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			percent, err := parseFloat(args[1], "percent")
			if err != nil {
				return err
			}
			_, transition, err := brightnessAndTransitionFlags(cmd)
			if err != nil {
				return err
			}
			client, id, err := resolveTarget(cmd, target, args[0])
			if err != nil {
				return err
			}
			resp, err := hue.SetBrightness(cmd.Context(), target(client), id, percent, transition)
			if err != nil {
				return err
			}
			return writeResponse(cmd, resp)
		},
	}
	addTransitionFlag(brightnessCmd)

	colorCmd := &cobra.Command{
		Use:   "color <target> <hex>",
		Short: "Set a color from a #rgb or #rrggbb hex string",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			brightness, transition, err := brightnessAndTransitionFlags(cmd)
			if err != nil {
				return err
			}
			client, id, err := resolveTarget(cmd, target, args[0])
			if err != nil {
				return err
			}
			resp, err := hue.SetHex(cmd.Context(), target(client), id, args[1], brightness, transition)
			if err != nil {
				return err
			}
			return writeResponse(cmd, resp)
		},
	}
	addBrightnessFlag(colorCmd)
	addTransitionFlag(colorCmd)

	temperatureCmd := &cobra.Command{
		Use:   "temperature <target> <kelvin>",
		Short: "Set a color temperature in kelvin",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			kelvin, err := parseInt(args[1], "kelvin")
			if err != nil {
				return err
			}
			brightness, transition, err := brightnessAndTransitionFlags(cmd)
			if err != nil {
				return err
			}
			client, id, err := resolveTarget(cmd, target, args[0])
			if err != nil {
				return err
			}
			resp, err := hue.SetColorTemperature(cmd.Context(), target(client), id, kelvin, brightness, transition)
			if err != nil {
				return err
			}
			return writeResponse(cmd, resp)
		},
	}
	addBrightnessFlag(temperatureCmd)
	addTransitionFlag(temperatureCmd)

	identifyCmd := &cobra.Command{
		Use:   "identify <target>",
		Short: "Make the target breathe so it can be identified",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, id, err := resolveTarget(cmd, target, args[0])
			if err != nil {
				return err
			}
			resp, err := hue.Identify(cmd.Context(), target(client), id)
			if err != nil {
				return err
			}
			return writeResponse(cmd, resp)
		},
	}

	parent.AddCommand(onCmd, offCmd, toggleCmd, brightnessCmd, colorCmd, temperatureCmd, identifyCmd)
}

func resolveTarget(cmd *cobra.Command, target func(*hue.Client) controlTarget, arg string) (*hue.Client, string, error) {
	client, err := newClient()
	if err != nil {
		return nil, "", err
	}
	id, err := target(client).ResolveTarget(cmd.Context(), arg)
	if err != nil {
		return nil, "", err
	}
	return client, id, nil
}

func addBrightnessFlag(cmd *cobra.Command) {
	cmd.Flags().Float64P("brightness", "b", 0, "Brightness in percent")
}

func addTransitionFlag(cmd *cobra.Command) {
	cmd.Flags().Float64("transition", 0, "Fade duration in seconds")
}

func brightnessAndTransitionFlags(cmd *cobra.Command) (brightness, transition *float64, err error) {
	if cmd.Flags().Lookup("brightness") != nil && cmd.Flags().Changed("brightness") {
		v, err := cmd.Flags().GetFloat64("brightness")
		if err != nil {
			return nil, nil, err
		}
		brightness = &v
	}
	if cmd.Flags().Lookup("transition") != nil && cmd.Flags().Changed("transition") {
		v, err := cmd.Flags().GetFloat64("transition")
		if err != nil {
			return nil, nil, err
		}
		transition = &v
	}
	return brightness, transition, nil
}
