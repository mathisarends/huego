package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

func entertainmentStreamCommand() *cobra.Command {
	command := &cobra.Command{Use: "stream <target>", Short: "Play finite RGB frames over DTLS, then release the area", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		body, err := readPayload(cmd)
		if err != nil {
			return err
		}
		var input struct {
			Frames []hue.EntertainmentFrame `json:"frames"`
		}
		if err := json.Unmarshal(body, &input); err != nil {
			return err
		}
		fps, _ := cmd.Flags().GetInt("fps")
		if fps < 1 || fps > 60 || len(input.Frames) == 0 {
			return fmt.Errorf("provide frames and --fps between 1 and 60")
		}
		for index, frame := range input.Frames {
			if _, err := hue.EncodeEntertainmentFrame("00000000-0000-0000-0000-000000000000", byte(index%256), frame); err != nil {
				return err
			}
		}
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		id := args[0]
		var client *hue.Client
		if !dryRun || !hue.IsUUID(id) {
			client, err = newClient()
			if err != nil {
				return err
			}
			id, err = resolveResourceTarget(cmd, client, "entertainment_configuration", id, false)
			if err != nil {
				return err
			}
		}
		if dryRun {
			return writeStructured(cmd, map[string]any{"area_id": id, "frames": len(input.Frames), "fps": fps, "duration_seconds": float64(len(input.Frames)) / float64(fps), "transport": "DTLS 1.2 / UDP 2100", "channels": "not_checked", "writes": false})
		}
		area, err := readResource(cmd, client, "entertainment_configuration", id)
		if err != nil {
			return err
		}
		if decodeField[string](area, "status") == "active" {
			return &hue.Problem{Code: "resource_busy", Message: "entertainment area is already active; stop the existing stream explicitly first"}
		}
		channels := map[string]bool{}
		for _, channel := range decodeField[[]hue.Resource](area, "channels") {
			channels[strconv.Itoa(decodeField[int](channel, "channel_id"))] = true
		}
		for _, frame := range input.Frames {
			for channel := range frame {
				if !channels[channel] {
					return fmt.Errorf("channel %s is not part of this entertainment area", channel)
				}
			}
		}
		sent, err := client.StreamEntertainment(cmd.Context(), id, input.Frames, fps)
		if err != nil {
			return &hue.Problem{Code: "stream_failed", Message: err.Error(), Details: map[string]any{"frames_sent": sent, "area_id": id}}
		}
		return writeStructured(cmd, map[string]any{"frames_sent": sent, "area_id": id, "released": true})
	}}
	addPayloadFlags(command)
	command.Flags().Int("fps", 25, "Frames per second (1-60)")
	command.Flags().Bool("dry-run", false, "Validate frame values and preview playback without streaming")
	return command
}
