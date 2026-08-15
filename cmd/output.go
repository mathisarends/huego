package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func writeJSON(cmd *cobra.Command, value any) (bool, error) {
	enabled, _ := cmd.Flags().GetBool("json")
	if !enabled {
		return false, nil
	}
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetEscapeHTML(false)
	return true, encoder.Encode(value)
}

func reportEmpty(cmd *cobra.Command, count int) {
	if count == 0 {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "No results.")
	}
}

func init() {
	rootCmd.PersistentFlags().Bool("json", false, "write machine-readable JSON")
}
