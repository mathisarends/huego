package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/mathisarends/huego/internal/omarchy"
	"github.com/spf13/cobra"
)

type omarchyResult struct {
	DryRun          bool             `json:"dry_run"`
	Changes         []omarchy.Change `json:"changes"`
	WaybarRestarted bool             `json:"waybar_restarted"`
}

// restartWaybar reports false when Omarchy's restart helper is missing, so
// the user is told to restart Waybar instead.
var restartWaybar = func(ctx context.Context) (bool, error) {
	path, err := lookPath("omarchy-restart-waybar")
	if err != nil {
		return false, nil
	}
	return true, exec.CommandContext(ctx, path).Run()
}

var omarchyConfigHome = os.UserConfigDir

var omarchyCmd = &cobra.Command{
	Use:   "omarchy",
	Short: "Add huego's keybindings and Waybar module to an Omarchy desktop",
	Long: "Add huego's Hyprland keybindings and Waybar module to an Omarchy desktop, or\n" +
		"remove them again. Files are looked up in $XDG_CONFIG_HOME, else ~/.config.\n" +
		"Every edit sits between huego markers; the first edit of a file keeps the\n" +
		"original as <file>.huego.bak.",
}

var omarchyInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Write the keybindings and add the Waybar module",
	Long: "Write hypr/huego.conf and source it from hypr/hyprland.conf, add custom/hue to\n" +
		"waybar/config.jsonc and its style to waybar/style.css. Running it again updates\n" +
		"huego's sections in place. Hyprland reloads by itself; Waybar is restarted with\n" +
		"omarchy-restart-waybar when that is on PATH.",
	Example: "  huego omarchy install\n" +
		"  huego omarchy install --dry-run\n" +
		"  huego omarchy install --modifier \"SUPER SHIFT\"",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		modifier, _ := cmd.Flags().GetString("modifier")
		return runOmarchy(cmd, func(o omarchy.Options) ([]omarchy.Change, error) {
			o.Modifier = modifier
			return omarchy.Install(o)
		})
	},
}

var omarchyUninstallCmd = &cobra.Command{
	Use:     "uninstall",
	Short:   "Remove everything omarchy install added",
	Example: "  huego omarchy uninstall\n  huego omarchy uninstall --dry-run",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runOmarchy(cmd, omarchy.Uninstall)
	},
}

func runOmarchy(cmd *cobra.Command, apply func(omarchy.Options) ([]omarchy.Change, error)) error {
	home, err := omarchyConfigHome()
	if err != nil {
		return err
	}
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	changes, applyErr := apply(omarchy.Options{ConfigHome: home, DryRun: dryRun})

	result := omarchyResult{DryRun: dryRun, Changes: changes}
	if result.Changes == nil {
		result.Changes = []omarchy.Change{}
	}
	if applyErr == nil && !dryRun && omarchy.HasWaybarChanges(changes) {
		restarted, err := restartWaybar(cmd.Context())
		if err != nil {
			applyErr = fmt.Errorf("restart Waybar: %w", err)
		}
		result.WaybarRestarted = restarted && err == nil
		if !restarted {
			defer func() { _, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Restart Waybar to load the change.") }()
		}
	}
	if allSkipped(changes) && applyErr == nil {
		applyErr = fmt.Errorf("nothing changed in %s: every file was skipped", home)
	}

	if written, err := writeJSON(cmd, result); written || err != nil {
		return errors.Join(applyErr, err)
	}
	if dryRun {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Dry run: nothing was written.")
	}
	rows := make([][]string, len(changes))
	for i, c := range changes {
		rows[i] = []string{c.File, string(c.Action), c.Note}
	}
	writeTable(cmd, []string{"FILE", "ACTION", "NOTE"}, rows)
	return applyErr
}

func allSkipped(changes []omarchy.Change) bool {
	for _, c := range changes {
		if c.Action != omarchy.Skipped {
			return false
		}
	}
	return len(changes) > 0
}

func init() {
	omarchyInstallCmd.Flags().String("modifier", omarchy.DefaultModifier, "Hyprland modifier for every keybinding")
	for _, c := range []*cobra.Command{omarchyInstallCmd, omarchyUninstallCmd} {
		c.Flags().Bool("dry-run", false, "show what would change without writing anything")
		omarchyCmd.AddCommand(c)
	}
	rootCmd.AddCommand(omarchyCmd)
}
