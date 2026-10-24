package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

// controlTarget is satisfied by *hue.Lights and by *hue.Rooms, *hue.Zones,
// and *hue.Home, which act through their grouped_light service instead of
// light by light.
type controlTarget interface {
	hue.Controllable
	ResolveTarget(ctx context.Context, target string) (string, error)
}

type controlRequest struct {
	ctx        context.Context
	target     hue.Controllable
	id         string
	args       []string
	brightness *float64
	transition *float64
}

type controlResult = hue.APIResponse[hue.ResourceIdentifier]

// controlAction is one verb of the vocabulary shared by light, room, zone,
// and home. validate runs before any bridge request so bad input never
// causes a partial update.
type controlAction struct {
	use        string
	aliases    []string
	short      string
	long       string
	args       []string
	optional   int
	brightness bool
	transition bool
	validate   func(args []string) error
	run        func(r controlRequest) (controlResult, error)
}

var controlActions = []controlAction{
	{
		use:        "on",
		short:      "Turn on",
		brightness: true,
		transition: true,
		run: func(r controlRequest) (controlResult, error) {
			return hue.TurnOn(r.ctx, r.target, r.id, r.brightness, nil, r.transition)
		},
	},
	{
		use:        "off",
		short:      "Turn off",
		transition: true,
		run: func(r controlRequest) (controlResult, error) {
			return hue.TurnOff(r.ctx, r.target, r.id, r.transition)
		},
	},
	{
		use:        "toggle",
		short:      "Turn off if on, otherwise on",
		transition: true,
		run: func(r controlRequest) (controlResult, error) {
			return hue.Toggle(r.ctx, r.target, r.id, r.transition)
		},
	},
	{
		use:        "brightness",
		aliases:    []string{"bri"},
		short:      "Set brightness from 0 to 100; 0 turns off",
		args:       []string{"<percent>"},
		transition: true,
		validate: func(args []string) error {
			_, err := parsePercentage(args[0], "percent")
			return err
		},
		run: func(r controlRequest) (controlResult, error) {
			percent, _ := parsePercentage(r.args[0], "percent")
			return hue.SetBrightness(r.ctx, r.target, r.id, percent, r.transition)
		},
	},
	{
		use:      "brighten",
		aliases:  []string{"up"},
		short:    "Raise brightness by a step (default 10) and turn on",
		args:     []string{"[step]"},
		optional: 1,
		validate: validateStep,
		run: func(r controlRequest) (controlResult, error) {
			return hue.Brighten(r.ctx, r.target, r.id, stepArg(r.args))
		},
	},
	{
		use:      "dim",
		aliases:  []string{"down"},
		short:    "Lower brightness by a step (default 10), stopping at the minimum",
		args:     []string{"[step]"},
		optional: 1,
		validate: validateStep,
		run: func(r controlRequest) (controlResult, error) {
			return hue.Dim(r.ctx, r.target, r.id, stepArg(r.args))
		},
	},
	{
		use:        "color",
		aliases:    []string{"colour"},
		short:      "Set a color by name, #rgb, or #rrggbb",
		long:       "Set a color by name, #rgb, or #rrggbb.\n\nColor names: " + strings.Join(colorNames(), ", "),
		args:       []string{"<color>"},
		brightness: true,
		transition: true,
		validate: func(args []string) error {
			_, err := hue.ParseColor(args[0])
			return err
		},
		run: func(r controlRequest) (controlResult, error) {
			rgb, _ := hue.ParseColor(r.args[0])
			return hue.SetRGB(r.ctx, r.target, r.id, rgb, r.brightness, r.transition)
		},
	},
	{
		use:        "temperature",
		aliases:    []string{"temp", "ct"},
		short:      fmt.Sprintf("Set a white color temperature in kelvin (%d-%d)", hue.KelvinMinimum, hue.KelvinMaximum),
		args:       []string{"<kelvin>"},
		brightness: true,
		transition: true,
		validate: func(args []string) error {
			_, err := parseKelvin(args[0])
			return err
		},
		run: func(r controlRequest) (controlResult, error) {
			kelvin, _ := parseKelvin(r.args[0])
			return hue.SetColorTemperature(r.ctx, r.target, r.id, kelvin, r.brightness, r.transition)
		},
	},
	{
		use:   "identify",
		short: "Make the lights breathe so they can be found physically",
		run: func(r controlRequest) (controlResult, error) {
			return hue.Identify(r.ctx, r.target, r.id)
		},
	},
}

// addControlCommands attaches the shared control vocabulary to a parent
// command. With targeted, every command takes a name or UUID as its first
// argument; without, the target is implicit (home).
func addControlCommands(parent *cobra.Command, target func(*hue.Client) controlTarget, targeted bool) {
	for _, action := range controlActions {
		parent.AddCommand(newControlCommand(action, target, targeted))
	}
}

func newControlCommand(action controlAction, target func(*hue.Client) controlTarget, targeted bool) *cobra.Command {
	positional := action.args
	if targeted {
		positional = append([]string{"<target>"}, positional...)
	}
	maxArgs := len(positional)

	cmd := &cobra.Command{
		Use:     strings.Join(append([]string{action.use}, positional...), " "),
		Aliases: action.aliases,
		Short:   action.short,
		Long:    action.long,
		Args:    cobra.RangeArgs(maxArgs-action.optional, maxArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			label := cmd.Parent().Name()
			if targeted {
				label, args = args[0], args[1:]
			}
			if action.validate != nil {
				if err := action.validate(args); err != nil {
					return err
				}
			}
			brightness, transition, err := brightnessAndTransitionFlags(cmd)
			if err != nil {
				return err
			}
			client, id, err := resolveTarget(cmd, target, label)
			if err != nil {
				return err
			}
			resp, err := action.run(controlRequest{
				ctx: cmd.Context(), target: target(client), id: id, args: args,
				brightness: brightness, transition: transition,
			})
			if err != nil {
				return err
			}
			return writeResponse(cmd, label, resp)
		},
	}
	if action.brightness {
		addBrightnessFlag(cmd)
	}
	if action.transition {
		addTransitionFlag(cmd)
	}
	if len(action.args) > 0 {
		cmd.SetFlagErrorFunc(readableNumericFlagError(strings.Trim(action.args[0], "<>[]")))
	}
	return cmd
}

func validateStep(args []string) error {
	if len(args) == 0 {
		return nil
	}
	step, err := parsePercentage(args[0], "step")
	if err != nil {
		return err
	}
	if step == 0 {
		return fmt.Errorf("invalid step %q: must be greater than 0", args[0])
	}
	return nil
}

func stepArg(args []string) float64 {
	if len(args) == 0 {
		return 0
	}
	step, _ := parsePercentage(args[0], "step")
	return step
}

func colorNames() []string {
	names := hue.ColorNames()
	sort.Strings(names)
	return names
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
	cmd.Flags().Float64P("brightness", "b", 0, "brightness in percent (0-100)")
}

func addTransitionFlag(cmd *cobra.Command) {
	cmd.Flags().Float64P("transition", "t", 0, "fade duration in seconds")
}

func brightnessAndTransitionFlags(cmd *cobra.Command) (brightness, transition *float64, err error) {
	if cmd.Flags().Lookup("brightness") != nil && cmd.Flags().Changed("brightness") {
		v, err := cmd.Flags().GetFloat64("brightness")
		if err != nil {
			return nil, nil, err
		}
		if v < 0 || v > 100 {
			return nil, nil, fmt.Errorf("brightness must be between 0 and 100")
		}
		brightness = &v
	}
	if cmd.Flags().Lookup("transition") != nil && cmd.Flags().Changed("transition") {
		v, err := cmd.Flags().GetFloat64("transition")
		if err != nil {
			return nil, nil, err
		}
		if v < 0 {
			return nil, nil, fmt.Errorf("transition must not be negative")
		}
		transition = &v
	}
	return brightness, transition, nil
}
