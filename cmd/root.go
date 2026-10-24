package cmd

import (
	"context"
	"os"
	"os/signal"
	"slices"
	"syscall"

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

// wantsJSON also scans the raw arguments because flag parsing stops at the
// first bad flag, which may come before --json.
func wantsJSON(cmd *cobra.Command) bool {
	return jsonEnabled(cmd) || slices.Contains(os.Args[1:], "--json")
}

func Execute() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	prepareCommands()
	cmd, err := rootCmd.ExecuteContextC(ctx)
	if err != nil {
		stop()
		os.Exit(reportError(os.Stderr, err, wantsJSON(cmd)))
	}
}
