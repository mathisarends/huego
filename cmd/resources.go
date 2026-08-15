package cmd

import (
	"fmt"

	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

type resourceRow struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state,omitempty"`
}

func writeResourceRows(cmd *cobra.Command, rows []resourceRow) error {
	if written, err := writeJSON(cmd, rows); written || err != nil {
		return err
	}
	for _, row := range rows {
		if row.State != "" {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  (%s)\n", row.ID, row.Name, row.State)
		} else {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s  %s\n", row.ID, row.Name)
		}
	}
	reportEmpty(cmd, len(rows))
	return nil
}

func lightRows(lights []hue.Light) []resourceRow {
	rows := make([]resourceRow, len(lights))
	for i, l := range lights {
		state := "off"
		if l.IsOn() {
			state = "on"
		}
		rows[i] = resourceRow{ID: l.ID, Name: l.Name(), State: state}
	}
	return rows
}

func groupRows(groups []hue.Group) []resourceRow {
	rows := make([]resourceRow, len(groups))
	for i, g := range groups {
		rows[i] = resourceRow{ID: g.ID, Name: g.Name()}
	}
	return rows
}

func sceneRows(scenes []hue.Scene) []resourceRow {
	rows := make([]resourceRow, len(scenes))
	for i, s := range scenes {
		state := ""
		if s.Status != nil && s.Status.Active != nil {
			state = string(*s.Status.Active)
		}
		rows[i] = resourceRow{ID: s.ID, Name: s.Name(), State: state}
	}
	return rows
}

func writeResponse(cmd *cobra.Command, resp hue.APIResponse[hue.ResourceIdentifier]) error {
	if written, err := writeJSON(cmd, resp); written || err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Updated %d resource(s).\n", len(resp.Data))
	return nil
}
