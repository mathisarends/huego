package cmd

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

type picker struct {
	name string
	args []string
}

// knownPickers are tried in order when neither --picker nor HUEGO_PICKER
// names one. Walker comes first because Omarchy ships it.
var knownPickers = []picker{
	{"walker", []string{"--dmenu", "-p", "Scene…"}},
	{"fuzzel", []string{"--dmenu", "-p", "Scene: "}},
	{"rofi", []string{"-dmenu", "-i", "-p", "Scene"}},
	{"wofi", []string{"--dmenu", "-p", "Scene"}},
	{"fzf", []string{"--prompt", "Scene> "}},
}

var lookPath = exec.LookPath

func pickerCommand(configured string) ([]string, error) {
	if configured == "" {
		configured = os.Getenv("HUEGO_PICKER")
	}
	if configured != "" {
		return strings.Fields(configured), nil
	}
	for _, p := range knownPickers {
		if _, err := lookPath(p.name); err == nil {
			return append([]string{p.name}, p.args...), nil
		}
	}
	return nil, errors.New("no picker found: install walker, fuzzel, rofi, wofi, or fzf, or pass --picker")
}

// runPicker offers choices, one per line, and returns the chosen line, or ""
// when the picker was dismissed. Pickers signal a dismissal with a non-zero
// exit status (fzf and rofi) or by printing nothing (walker).
var runPicker = func(ctx context.Context, argv []string, choices string) (string, error) {
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)
	command.Stdin = strings.NewReader(choices)
	command.Stderr = os.Stderr
	out, err := command.Output()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return "", nil
	}
	return strings.TrimRight(string(out), "\r\n"), err
}

// notify shows err as a desktop notification, for keybindings where nobody
// reads stderr.
var notify = func(message string) {
	_ = exec.Command("notify-send", "--app-name=huego", "Hue", message).Run()
}

var scenePickCmd = &cobra.Command{
	Use:   "pick",
	Short: "Choose a scene in a picker such as Walker and activate it",
	Long: "Choose a scene in a dmenu-style picker and activate it. The picker is --picker,\n" +
		"else $HUEGO_PICKER, else the first of walker, fuzzel, rofi, wofi, or fzf on PATH.\n" +
		"Dismissing the picker activates nothing and exits 0.",
	Example: "  huego scene pick\n" +
		"  huego scene pick --group Office --notify\n" +
		"  huego scene pick --picker \"fzf --height 10\"",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := pickScene(cmd)
		if shouldNotify, _ := cmd.Flags().GetBool("notify"); err != nil && shouldNotify {
			notify(err.Error())
		}
		return err
	},
}

func pickScene(cmd *cobra.Command) error {
	brightness, transition, err := brightnessAndTransitionFlags(cmd)
	if err != nil {
		return err
	}
	activation := hue.SceneActivation{Brightness: brightness, Transition: transition}
	activation.Dynamic, _ = cmd.Flags().GetBool("dynamic")
	configured, _ := cmd.Flags().GetString("picker")
	argv, err := pickerCommand(configured)
	if err != nil {
		return err
	}

	client, err := newClient()
	if err != nil {
		return err
	}
	scenes, err := client.Scenes.ListWithGroupNames(cmd.Context())
	if err != nil {
		return err
	}
	if group, _ := cmd.Flags().GetString("group"); group != "" {
		if scenes, err = inGroup(scenes, group, "scenes", func(s hue.Scene) string { return s.GroupName }); err != nil {
			return err
		}
	}
	choices := make([]string, len(scenes))
	for i, scene := range scenes {
		choices[i] = scene.QualifiedName()
	}

	chosen, err := runPicker(cmd.Context(), argv, strings.Join(choices, "\n")+"\n")
	if err != nil || chosen == "" {
		return err
	}
	id, err := client.Scenes.ResolveTarget(cmd.Context(), chosen)
	if err != nil {
		return err
	}
	resp, err := client.Scenes.Activate(cmd.Context(), id, activation)
	if err != nil {
		return err
	}
	return writeResponse(cmd, chosen, resp)
}

func init() {
	addBrightnessFlag(scenePickCmd)
	addTransitionFlag(scenePickCmd)
	scenePickCmd.Flags().Bool("dynamic", false, "start the scene's dynamic palette")
	scenePickCmd.Flags().String("group", "", "only offer scenes of this room or zone")
	scenePickCmd.Flags().String("picker", "", "picker command that reads choices on stdin, e.g. \"walker --dmenu\"")
	scenePickCmd.Flags().Bool("notify", false, "also report errors with notify-send, for keybindings")
	sceneCmd.AddCommand(scenePickCmd)
}
