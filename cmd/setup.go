package cmd

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/mathisarends/huego/internal/config"
	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Discover a bridge and register the application interactively",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		ctx := cmd.Context()

		fmt.Fprintln(out, "Hue Bridge Setup")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Searching for bridges on your network...")
		bridges, err := hue.DiscoverBridges(ctx)
		if err != nil {
			return err
		}

		bridge, err := selectBridge(cmd, bridges)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "\nUsing bridge at %s\n\n", bridge.InternalIPAddress)
		fmt.Fprintln(out, "Press the link button on your Hue Bridge, then hit Enter.")
		_, _ = bufio.NewReader(cmd.InOrStdin()).ReadString('\n')

		fmt.Fprintln(out, "Registering app key...")
		app, err := hue.RegisterAppKey(ctx, bridge.InternalIPAddress)
		if err != nil {
			return err
		}

		saved := config.Bridge{IP: bridge.InternalIPAddress, AppKey: app.AppKey, ClientKey: app.ClientKey}
		if err := saveBridge(saved); err != nil {
			return err
		}
		printSetupResult(out, saved)
		return nil
	},
}

// saveBridge is a var so tests can substitute a fake bridge store.
var saveBridge = config.SaveBridge

func selectBridge(cmd *cobra.Command, bridges []hue.DiscoveredBridge) (hue.DiscoveredBridge, error) {
	if len(bridges) == 1 {
		return bridges[0], nil
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Found %d bridges:\n\n", len(bridges))
	for i, b := range bridges {
		fmt.Fprintf(out, "  [%d] %s  (%s)\n", i+1, b.InternalIPAddress, b.ID)
	}
	fmt.Fprintln(out)

	reader := bufio.NewReader(cmd.InOrStdin())
	for {
		fmt.Fprintf(out, "Select a bridge (1-%d): ", len(bridges))
		line, _ := reader.ReadString('\n')
		choice, err := strconv.Atoi(strings.TrimSpace(line))
		if err == nil && choice >= 1 && choice <= len(bridges) {
			return bridges[choice-1], nil
		}
		fmt.Fprintln(out, "Invalid choice, try again.")
	}
}

func printSetupResult(out io.Writer, bridge config.Bridge) {
	fmt.Fprintln(out, "\nSetup complete. huego stores these values in its config directory:")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "  bridge_ip = %s\n", bridge.IP)
	fmt.Fprintf(out, "  app_key   = %s\n", bridge.AppKey)
	if bridge.ClientKey != "" {
		fmt.Fprintf(out, "  client_key = %s\n", bridge.ClientKey)
	}
	fmt.Fprintln(out, "\nThese keys control your bridge - keep them out of version control.")
	fmt.Fprintln(out, "You can also set HUEGO_BRIDGE_IP / HUEGO_APP_KEY / HUEGO_CLIENT_KEY instead.")
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
