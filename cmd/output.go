package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func jsonEnabled(cmd *cobra.Command) bool {
	enabled, _ := cmd.Flags().GetBool("json")
	return enabled
}

func writeJSON(cmd *cobra.Command, value any) (bool, error) {
	if !jsonEnabled(cmd) {
		return false, nil
	}
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetEscapeHTML(false)
	return true, encoder.Encode(value)
}

// listing describes how a list command renders its items as a table and,
// with --names, as one name per line for dmenu-style pickers.
type listing[T any] struct {
	headers []string
	row     func(T) []string
	name    func(T) string
}

func writeList[T any](cmd *cobra.Command, items []T, l listing[T]) error {
	if items == nil {
		items = []T{}
	}
	if written, err := writeJSON(cmd, items); written || err != nil {
		return err
	}
	if names, _ := cmd.Flags().GetBool("names"); names {
		for _, item := range items {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), l.name(item))
		}
		return nil
	}
	if len(items) == 0 {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "No results.")
		return nil
	}
	rows := make([][]string, len(items))
	for i, item := range items {
		rows[i] = l.row(item)
	}
	writeTable(cmd, l.headers, rows)
	return nil
}

func writeTable(cmd *cobra.Command, headers []string, rows [][]string) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, strings.Join(headers, "\t"))
	for _, row := range rows {
		_, _ = fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	_ = w.Flush()
}

// field is one "Label: value" line of a detail view; empty values are skipped.
type field struct {
	label string
	value string
}

func writeDetails(cmd *cobra.Command, value any, fields []field) error {
	if written, err := writeJSON(cmd, value); written || err != nil {
		return err
	}
	width := 0
	for _, f := range fields {
		width = max(width, len(f.label))
	}
	for _, f := range fields {
		if f.value == "" {
			continue
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-*s  %s\n", width+1, f.label+":", f.value)
	}
	return nil
}

func addNamesFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("names", false, "print one name per line, for pickers like walker --dmenu or fzf")
}

func init() {
	rootCmd.PersistentFlags().Bool("json", false, "write machine-readable JSON")
}
