package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is set at build time via -ldflags "-X ...cmd.version=...".
var version = "dev"

var rootCmd = &cobra.Command{
	Use:           "huego",
	Short:         "A Philips Hue CLI backed by the local CLIP v2 API",
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
