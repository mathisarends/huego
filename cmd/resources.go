package cmd

import (
	"fmt"
	"strings"

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
		state := ""
		if g.AggregateOn != nil {
			state = "off"
			if g.AggregateOn.On {
				state = "on"
			}
		}
		rows[i] = resourceRow{ID: g.ID, Name: g.Name(), State: state}
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
		rows[i] = resourceRow{ID: s.ID, Name: s.QualifiedName(), State: state}
	}
	return rows
}

func writeResponse(cmd *cobra.Command, target string, resp hue.APIResponse[hue.ResourceIdentifier]) error {
	written, err := writeJSON(cmd, resp)
	if err != nil {
		return err
	}
	if len(resp.Errors) > 0 {
		descriptions := make([]string, len(resp.Errors))
		for i, responseError := range resp.Errors {
			descriptions[i] = responseError.Description
		}
		return fmt.Errorf("bridge could not fully update %q: %s", target, strings.Join(descriptions, "; "))
	}
	if written {
		return nil
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Updated %q (%d resource(s)).\n", target, len(resp.Data))
	return nil
}
