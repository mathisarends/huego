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

type setupResult struct {
	BridgeIP   string `json:"bridge_ip"`
	AppKey     string `json:"app_key"`
	ClientKey  string `json:"client_key,omitempty"`
	ConfigPath string `json:"config_path,omitempty"`
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Find a bridge and register huego with it",
	Long: "Find a bridge and register huego with it. Without --ip the bridge is looked up\n" +
		"through discovery.meethue.com; control stays local afterwards. Press the bridge's\n" +
		"link button when asked: setup waits up to 60 seconds for it.\n\n" +
		"Progress goes to stderr and the result to stdout, so --json output can be captured.",
	Example: "  huego setup\n  huego setup --ip 192.168.1.20 --json",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		progress := cmd.ErrOrStderr()

		ip, _ := cmd.Flags().GetString("ip")
		if ip == "" {
			_, _ = fmt.Fprintln(progress, "Searching for bridges via discovery.meethue.com...")
			bridges, err := hue.DiscoverBridges(ctx)
			if err != nil {
				return err
			}
			bridge, err := selectBridge(cmd, bridges)
			if err != nil {
				return err
			}
			ip = bridge.InternalIPAddress
		}

		_, _ = fmt.Fprintf(progress, "Using bridge at %s.\nPress the link button on the bridge now (waiting up to 60 seconds)...\n", ip)
		app, err := hue.RegisterAppKey(ctx, ip)
		if err != nil {
			return err
		}

		saved := config.Bridge{IP: ip, AppKey: app.AppKey, ClientKey: app.ClientKey}
		if err := saveBridge(saved); err != nil {
			return err
		}
		result := setupResult{BridgeIP: saved.IP, AppKey: saved.AppKey, ClientKey: saved.ClientKey}
		if path, err := config.BridgePath(); err == nil {
			result.ConfigPath = path
		}
		if written, err := writeJSON(cmd, result); written || err != nil {
			return err
		}
		printSetupResult(cmd.OutOrStdout(), result)
		return nil
	},
}

// saveBridge is a var so tests can substitute a fake bridge store.
var saveBridge = config.SaveBridge

func selectBridge(cmd *cobra.Command, bridges []hue.DiscoveredBridge) (hue.DiscoveredBridge, error) {
	if len(bridges) == 1 {
		return bridges[0], nil
	}

	out := cmd.ErrOrStderr()
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
			return hue.DiscoveredBridge{}, usagef("found %d bridges and no choice on stdin; pass one with --ip", len(bridges))
		}
		_, _ = fmt.Fprintln(out, "Invalid choice, try again.")
	}
}

func printSetupResult(out io.Writer, result setupResult) {
	_, _ = fmt.Fprintln(out, "Setup complete.")
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintf(out, "  bridge_ip  = %s\n", result.BridgeIP)
	_, _ = fmt.Fprintf(out, "  app_key    = %s\n", result.AppKey)
	if result.ClientKey != "" {
		_, _ = fmt.Fprintf(out, "  client_key = %s\n", result.ClientKey)
	}
	if result.ConfigPath != "" {
		_, _ = fmt.Fprintf(out, "\nSaved to %s.\n", result.ConfigPath)
	}
	_, _ = fmt.Fprintln(out, "These keys control your bridge - keep them out of version control.")
	_, _ = fmt.Fprintln(out, "HUEGO_BRIDGE_IP, HUEGO_APP_KEY, and HUEGO_CLIENT_KEY override the saved values.")
}

func init() {
	setupCmd.Flags().String("ip", "", "bridge address; skips cloud discovery")
	rootCmd.AddCommand(setupCmd)
}
