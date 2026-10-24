package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

// waybarStatus is Waybar's custom-module JSON ("return-type": "json").
type waybarStatus struct {
	Text       string `json:"text"`
	Alt        string `json:"alt"`
	Class      string `json:"class"`
	Percentage int    `json:"percentage"`
	Tooltip    string `json:"tooltip"`
}

const (
	waybarOn           = "on"
	waybarOff          = "off"
	waybarDisconnected = "disconnected"
)

// waybarDebounce coalesces the burst of events one scene or group change
// produces, so the bar re-renders once instead of once per light.
const waybarDebounce = 150 * time.Millisecond

// waybarSelection is the group the module reports on: a room, a zone, or,
// when neither flag is set, the whole home.
type waybarSelection struct {
	room string
	zone string
}

func (s waybarSelection) resolve(ctx context.Context, client *hue.Client) (groupTarget, string, error) {
	var target groupTarget
	var name string
	switch {
	case s.room != "":
		target, name = client.Rooms, s.room
	case s.zone != "":
		target, name = client.Zones, s.zone
	default:
		target = client.Home
	}
	id, err := target.ResolveTarget(ctx, name)
	return target, id, err
}

var waybarCmd = &cobra.Command{
	Use:   "waybar",
	Short: "Print light status as Waybar custom-module JSON",
	Long: "Print the state of the home, a room, or a zone as Waybar custom-module JSON\n" +
		"(text, alt, class, percentage, tooltip). class and alt are on, off, or disconnected.\n\n" +
		"With --watch the command keeps running and prints a new line on every change from\n" +
		"the bridge's event stream, so Waybar needs no polling interval. Output stays valid\n" +
		"JSON when the bridge is unreachable, and the stream reconnects on its own.",
	Example: "  huego waybar --watch\n  huego waybar --room Kitchen",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		room, _ := cmd.Flags().GetString("room")
		zone, _ := cmd.Flags().GetString("zone")
		watch, _ := cmd.Flags().GetBool("watch")
		selection := waybarSelection{room: room, zone: zone}
		out := &waybarWriter{w: cmd.OutOrStdout()}

		client, err := newClient()
		if err != nil {
			out.write(disconnectedStatus(err))
			return err
		}
		if watch {
			return watchWaybar(cmd.Context(), client, selection, out, cmd.ErrOrStderr())
		}
		status, err := waybarState(cmd.Context(), client, selection)
		if err != nil {
			out.write(disconnectedStatus(err))
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "error:", err)
			return nil
		}
		out.write(status)
		return nil
	},
}

type waybarWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (o *waybarWriter) write(status waybarStatus) {
	o.mu.Lock()
	defer o.mu.Unlock()
	encoder := json.NewEncoder(o.w)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(status)
}

func watchWaybar(ctx context.Context, client *hue.Client, selection waybarSelection, out *waybarWriter, errOut io.Writer) error {
	refresh := make(chan struct{}, 1)
	trigger := func() {
		select {
		case refresh <- struct{}{}:
		default:
		}
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-refresh:
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(waybarDebounce):
			}
			select {
			case <-refresh:
			default:
			}
			status, err := waybarState(ctx, client, selection)
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				status = disconnectedStatus(err)
			}
			out.write(status)
		}
	}()

	trigger()
	err := client.WatchEvents(ctx, hue.EventWatch{
		Connected: trigger,
		Event: func(event hue.Event) {
			if affectsLightState(event) {
				trigger()
			}
		},
		Disconnected: func(err error, retryIn time.Duration) {
			out.write(disconnectedStatus(err))
			_, _ = fmt.Fprintf(errOut, "event stream disconnected (%v); reconnecting in %s\n", err, retryIn)
		},
	})
	if err != nil {
		out.write(disconnectedStatus(err))
	}
	return err
}

func affectsLightState(event hue.Event) bool {
	for _, resource := range event.Resources() {
		switch resource.Type {
		case hue.ResourceTypeLight, hue.ResourceTypeGroupedLight, hue.ResourceTypeRoom, hue.ResourceTypeZone:
			return true
		}
	}
	return false
}

func waybarState(ctx context.Context, client *hue.Client, selection waybarSelection) (waybarStatus, error) {
	target, id, err := selection.resolve(ctx, client)
	if err != nil {
		return waybarStatus{}, err
	}
	group, err := target.GetWithState(ctx, id)
	if err != nil {
		return waybarStatus{}, err
	}

	name := group.Name()
	members := make([]string, 0, len(group.Lights))
	if group.Type == hue.ResourceTypeBridgeHome {
		name = hue.HomeName
		rooms, err := client.Rooms.ListWithState(ctx)
		if err != nil {
			return waybarStatus{}, err
		}
		for _, room := range rooms {
			members = append(members, stateLine(room.Name(), room.IsOn(), room.Brightness()))
		}
	} else {
		for _, light := range group.Lights {
			members = append(members, stateLine(light.Name(), light.IsOn(), light.Brightness()))
		}
	}

	tooltip := "<b>" + escapeMarkup(stateLine(name, group.IsOn(), group.Brightness())) + "</b>"
	if len(members) > 0 {
		tooltip += "\n" + escapeMarkup(strings.Join(members, "\n"))
	}

	status := waybarStatus{Text: waybarOff, Alt: waybarOff, Class: waybarOff, Tooltip: tooltip}
	if group.IsOn() {
		status.Alt, status.Class = waybarOn, waybarOn
		status.Text = percent(group.Brightness())
		if brightness := group.Brightness(); brightness != nil {
			status.Percentage = int(math.Round(*brightness))
		}
	}
	return status, nil
}

func stateLine(name string, on bool, brightness *float64) string {
	if !on {
		return name + ": off"
	}
	return name + ": on, " + percent(brightness)
}

func disconnectedStatus(err error) waybarStatus {
	return waybarStatus{
		Alt:     waybarDisconnected,
		Class:   waybarDisconnected,
		Tooltip: escapeMarkup("Hue bridge unreachable: " + err.Error()),
	}
}

// escapeMarkup escapes text for Waybar, which renders tooltips as Pango
// markup and rejects the whole tooltip on a stray "&" or "<".
func escapeMarkup(text string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(text)
}

func init() {
	waybarCmd.Flags().String("room", "", "report on this room instead of the whole home")
	waybarCmd.Flags().String("zone", "", "report on this zone instead of the whole home")
	waybarCmd.Flags().Bool("watch", false, "keep running and print a line on every change")
	waybarCmd.MarkFlagsMutuallyExclusive("room", "zone")
	rootCmd.AddCommand(waybarCmd)
}
