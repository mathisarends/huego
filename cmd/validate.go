package cmd

import (
	"strings"

	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

func newValidateCommand() *cobra.Command {
	command := &cobra.Command{Use: "validate <type>", Short: "Validate a request offline against the embedded schema", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		method, _ := cmd.Flags().GetString("method")
		body, err := readPayload(cmd)
		if err != nil {
			return err
		}
		if err := hue.ValidateRequest(args[0], strings.ToUpper(method), body); err != nil {
			return err
		}
		return writeStructured(cmd, planValidation{Schema: "valid", References: "not_checked", Capabilities: "not_checked"})
	}}
	command.Flags().String("method", "POST", "HTTP request method: POST or PUT")
	addPayloadFlags(command)
	return command
}

func init() {
	rootCmd.PersistentFlags().Bool("validate", false, "Validate native write payloads against the catalog; dry runs always validate unless --raw")
}
