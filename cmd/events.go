package cmd

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

// eventLine is one changed resource, flattened from the bridge's batched
// events so each output line stands on its own.
type eventLine struct {
	Time  time.Time        `json:"time"`
	Event string           `json:"event"`
	Type  hue.ResourceType `json:"type"`
	ID    string           `json:"id"`
	Name  string           `json:"name,omitempty"`
	Data  json.RawMessage  `json:"data"`
}

var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Stream live changes from the bridge until interrupted",
	Long: "Stream live changes from the bridge's event stream until interrupted, reconnecting\n" +
		"automatically. Each line is one changed resource; with --json each line is a JSON\n" +
		"object (newline-delimited JSON) with time, event, type, id, name, and data.",
	Example: "  huego events\n  huego events --type light,grouped_light --json",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		types, _ := cmd.Flags().GetStringSlice("type")
		client, err := newClient()
		if err != nil {
			return err
		}
		names, err := client.Names(cmd.Context())
		if err != nil {
			return err
		}

		var mu sync.Mutex
		return client.WatchEvents(cmd.Context(), hue.EventWatch{
			Event: func(event hue.Event) {
				mu.Lock()
				defer mu.Unlock()
				for _, resource := range event.Resources() {
					if len(types) > 0 && !slices.Contains(types, string(resource.Type)) {
						continue
					}
					writeEventLine(cmd, eventLine{
						Time: event.CreationTime, Event: event.Type, Type: resource.Type,
						ID: resource.ID, Name: names[resource.ID], Data: resource.Raw,
					})
				}
			},
			Disconnected: func(err error, retryIn time.Duration) {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "event stream disconnected (%v); reconnecting in %s\n", err, retryIn)
			},
		})
	},
}

func writeEventLine(cmd *cobra.Command, line eventLine) {
	if written, _ := writeJSON(cmd, line); written {
		return
	}
	label := line.Name
	if label == "" {
		label = line.ID
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  %s  %s  %s\n",
		line.Time.Local().Format(time.TimeOnly), line.Event, line.Type, label, summarizeChange(line.Data))
}

// summarizeChange renders the changed properties of an event resource,
// spelling out the common ones and passing the rest through as JSON.
func summarizeChange(data json.RawMessage) string {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return string(data)
	}
	for _, key := range []string{"id", "id_v1", "type", "owner"} {
		delete(fields, key)
	}

	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, summarizeField(key, fields[key]))
	}
	return strings.Join(parts, " ")
}

func summarizeField(key string, value json.RawMessage) string {
	switch key {
	case "on":
		var on hue.OnState
		if json.Unmarshal(value, &on) == nil {
			return onOff(on.On)
		}
	case "dimming":
		var dimming hue.DimmingState
		if json.Unmarshal(value, &dimming) == nil {
			return "brightness=" + percent(&dimming.Brightness)
		}
	case "color_temperature":
		var temperature hue.ColorTemperatureState
		if json.Unmarshal(value, &temperature) == nil && temperature.Mirek != nil {
			if kelvin, err := hue.MirekToKelvin(*temperature.Mirek); err == nil {
				return fmt.Sprintf("temperature=%dK", kelvin)
			}
		}
	case "color":
		var color hue.ColorState
		if json.Unmarshal(value, &color) == nil && color.XY != nil {
			return "color=" + hue.XYToHex(*color.XY, 100)
		}
	}
	return key + "=" + string(value)
}

func init() {
	eventsCmd.Flags().StringSlice("type", nil, "only show these resource types, e.g. light,grouped_light,scene")
	rootCmd.AddCommand(eventsCmd)
}
