package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"

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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		jsonOutput := jsonRequested(os.Args[1:])
		if jsonOutput {
			writeCommandError(os.Stderr, err)
		} else {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
		os.Exit(1)
	}
}

func jsonRequested(args []string) bool {
	enabled := false
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "--json" {
			enabled = true
		}
		if value, ok := strings.CutPrefix(arg, "--json="); ok {
			if parsed, err := strconv.ParseBool(value); err == nil {
				enabled = parsed
			}
		}
	}
	return enabled
}
