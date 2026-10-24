package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
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

func Execute() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
