package cmd

import (
	"encoding/json"

	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

type automationView struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Type          string          `json:"type,omitempty"`
	Enabled       bool            `json:"enabled"`
	Status        string          `json:"status,omitempty"`
	LastError     string          `json:"last_error,omitempty"`
	ScriptID      string          `json:"script_id"`
	Configuration json.RawMessage `json:"configuration,omitempty"`
}

func newAutomationView(a hue.Automation) automationView {
	return automationView{
		ID:            a.ID,
		Name:          a.GetName(),
		Type:          a.ScriptName,
		Enabled:       a.Enabled,
		Status:        a.Status,
		LastError:     a.LastError,
		ScriptID:      a.ScriptID,
		Configuration: a.Configuration,
	}
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

var automationListing = listing[automationView]{
	headers: []string{"NAME", "TYPE", "ENABLED", "STATUS", "ID"},
	row: func(v automationView) []string {
		return []string{v.Name, orDash(v.Type), yesNo(v.Enabled), orDash(v.Status), v.ID}
	},
	name: func(v automationView) string { return v.Name },
}

func (v automationView) fields() []field {
	return []field{
		{"Name", v.Name},
		{"ID", v.ID},
		{"Type", v.Type},
		{"Enabled", yesNo(v.Enabled)},
		{"Status", v.Status},
		{"Last error", v.LastError},
	}
}

var automationCmd = &cobra.Command{
	Use:     "automation",
	Aliases: []string{"automations"},
	Short:   "Inspect and switch Hue automations such as wake-ups, timers, and routines",
}

var automationListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List automations with their type and whether they are enabled",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		automations, err := client.Automations.ListWithScripts(cmd.Context())
		if err != nil {
			return err
		}
		return writeList(cmd, mapViews(automations, newAutomationView), automationListing)
	},
}

var automationGetCmd = &cobra.Command{
	Use:   "get <target>",
	Short: "Show one automation; --json includes its configuration",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		id, err := client.Automations.ResolveTarget(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		automations, err := client.Automations.ListWithScripts(cmd.Context())
		if err != nil {
			return err
		}
		automation, err := findByID(automations, id, "automation")
		if err != nil {
			return err
		}
		view := newAutomationView(automation)
		return writeDetails(cmd, view, view.fields())
	},
}

func newAutomationSwitchCommand(use, short string, enabled bool) *cobra.Command {
	return &cobra.Command{
		Use:   use + " <target>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newClient()
			if err != nil {
				return err
			}
			id, err := client.Automations.ResolveTarget(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			resp, err := client.Automations.SetEnabled(cmd.Context(), id, enabled)
			if err != nil {
				return err
			}
			return writeResponse(cmd, args[0], resp)
		},
	}
}

func init() {
	addNamesFlag(automationListCmd)
	automationCmd.AddCommand(
		automationListCmd,
		automationGetCmd,
		newAutomationSwitchCommand("enable", "Turn an automation on", true),
		newAutomationSwitchCommand("disable", "Turn an automation off without deleting it", false),
	)
	rootCmd.AddCommand(automationCmd)
}
