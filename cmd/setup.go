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
		nonInteractive, _ := cmd.Flags().GetBool("non-interactive")
		ip, _ := cmd.Flags().GetString("ip")
		profile, _ := cmd.Flags().GetString("bridge")
		jsonOutput, _ := cmd.Flags().GetBool("json")
		if nonInteractive {
			if ip == "" {
				return fmt.Errorf("--non-interactive requires --ip; use profile discover to find addresses")
			}
			app, err := hue.RegisterAppKey(ctx, ip)
			if err != nil {
				return err
			}
			saved := config.Bridge{IP: ip, AppKey: app.AppKey, ClientKey: app.ClientKey}
			if profile != "" {
				err = config.SaveProfile(profile, saved)
			} else {
				err = saveBridge(saved)
			}
			if err != nil {
				return err
			}
			return writeStructured(cmd, map[string]any{"paired": true, "bridge_ip": ip, "profile": profile})
		}
		if jsonOutput {
			return fmt.Errorf("setup --json requires --non-interactive --ip; press the bridge link button before running")
		}

		_, _ = fmt.Fprintln(out, "Hue Bridge Setup")
		_, _ = fmt.Fprintln(out)
		_, _ = fmt.Fprintln(out, "Discovery contacts discovery.meethue.com; bridge control remains local after setup.")
		_, _ = fmt.Fprintln(out, "Searching for bridges on your network...")
		bridges := []hue.DiscoveredBridge{{InternalIPAddress: ip}}
		var err error
		if ip == "" {
			bridges, err = hue.DiscoverBridges(ctx)
			if err != nil {
				return err
			}
		}

		bridge, err := selectBridge(cmd, bridges)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(out, "\nUsing bridge at %s\n\n", bridge.InternalIPAddress)
		_, _ = fmt.Fprintln(out, "Press the link button on your Hue Bridge, then hit Enter.")
		if _, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n'); err != nil {
			return fmt.Errorf("setup needs interactive input; use --non-interactive --ip: %w", err)
		}

		_, _ = fmt.Fprintln(out, "Registering app key...")
		app, err := hue.RegisterAppKey(ctx, bridge.InternalIPAddress)
		if err != nil {
			return err
		}

		saved := config.Bridge{IP: bridge.InternalIPAddress, AppKey: app.AppKey, ClientKey: app.ClientKey}
		if profile != "" {
			err = config.SaveProfile(profile, saved)
		} else {
			err = saveBridge(saved)
		}
		if err != nil {
			return err
		}
		printSetupResult(out, saved)
		return nil
	},
}

// saveBridge is a var so tests can substitute a fake bridge store.
var saveBridge = config.SaveBridge

func selectBridge(cmd *cobra.Command, bridges []hue.DiscoveredBridge) (hue.DiscoveredBridge, error) {
	if len(bridges) == 0 {
		return hue.DiscoveredBridge{}, fmt.Errorf("no bridges found; specify --ip")
	}
	if len(bridges) == 1 {
		return bridges[0], nil
	}

	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "Found %d bridges:\n\n", len(bridges))
	for i, b := range bridges {
		_, _ = fmt.Fprintf(out, "  [%d] %s  (%s)\n", i+1, b.InternalIPAddress, b.ID)
	}
	_, _ = fmt.Fprintln(out)

	reader := bufio.NewReader(cmd.InOrStdin())
	for {
		_, _ = fmt.Fprintf(out, "Select a bridge (1-%d): ", len(bridges))
		line, readErr := reader.ReadString('\n')
		choice, err := strconv.Atoi(strings.TrimSpace(line))
		if err == nil && choice >= 1 && choice <= len(bridges) {
			return bridges[choice-1], nil
		}
		if readErr != nil {
			return hue.DiscoveredBridge{}, fmt.Errorf("setup requires a bridge selection; use --non-interactive --ip: %w", readErr)
		}
		_, _ = fmt.Fprintln(out, "Invalid choice, try again.")
	}
}

func printSetupResult(out io.Writer, bridge config.Bridge) {
	_, _ = fmt.Fprintln(out, "\nSetup complete. huego stores these values in its config directory:")
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintf(out, "  bridge_ip = %s\n", bridge.IP)
	_, _ = fmt.Fprintf(out, "  app_key   = %s\n", bridge.AppKey)
	if bridge.ClientKey != "" {
		_, _ = fmt.Fprintf(out, "  client_key = %s\n", bridge.ClientKey)
	}
	_, _ = fmt.Fprintln(out, "\nThese keys control your bridge - keep them out of version control.")
	_, _ = fmt.Fprintln(out, "You can also set HUEGO_BRIDGE_IP / HUEGO_APP_KEY / HUEGO_CLIENT_KEY instead.")
}

func init() {
	setupCmd.Flags().String("ip", "", "Pair a specific bridge without cloud discovery")
	setupCmd.Flags().Bool("non-interactive", false, "Pair without prompts after a physical link-button press")
	rootCmd.AddCommand(setupCmd)
}
