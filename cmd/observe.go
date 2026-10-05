package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

var errObservationComplete = errors.New("observation complete")

func watchCommand() *cobra.Command {
	command := &cobra.Command{Use: "watch", Short: "Stream Hue events as JSON; optionally filter and reconnect", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		kind, _ := cmd.Flags().GetString("type")
		id, _ := cmd.Flags().GetString("id")
		limit, _ := cmd.Flags().GetInt("count")
		duration, _ := cmd.Flags().GetDuration("timeout")
		reconnect, _ := cmd.Flags().GetBool("reconnect")
		if limit < 0 || duration < 0 {
			return fmt.Errorf("count and timeout must be non-negative")
		}
		client, err := newClient()
		if err != nil {
			return err
		}
		ctx := cmd.Context()
		cancel := func() {}
		if duration > 0 {
			ctx, cancel = context.WithTimeout(ctx, duration)
		}
		defer cancel()
		lastID := ""
		seen := 0
		for {
			err = client.Events(ctx, lastID, func(message hue.EventMessage) error {
				lastID = message.ID
				var events []hue.Resource
				if err := json.Unmarshal(message.Data, &events); err != nil {
					return err
				}
				filtered := []hue.Resource{}
				for _, event := range events {
					data := decodeField[[]hue.Resource](event, "data")
					matching := []hue.Resource{}
					for _, resource := range data {
						if (kind == "" || decodeField[string](resource, "type") == kind) && (id == "" || resource.GetID() == id) {
							matching = append(matching, resource)
						}
					}
					if len(matching) > 0 {
						event["data"] = jsonValue(matching)
						filtered = append(filtered, event)
					}
				}
				if len(filtered) == 0 {
					return nil
				}
				message.Data = jsonValue(filtered)
				// Events are always one JSON object per line, independent of pretty-print preferences.
				if err := json.NewEncoder(cmd.OutOrStdout()).Encode(message); err != nil {
					return err
				}
				seen++
				if limit > 0 && seen >= limit {
					return errObservationComplete
				}
				return nil
			})
			if errors.Is(err, errObservationComplete) {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			var network net.Error
			var api *hue.APIError
			retryable := errors.Is(err, io.EOF) || errors.As(err, &network) || (errors.As(err, &api) && (api.StatusCode == 429 || api.StatusCode >= 500))
			if !reconnect || !retryable {
				return err
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
			}
		}
	}}
	command.Flags().String("type", "", "Only events for this native resource type")
	command.Flags().String("id", "", "Only events for this resource UUID")
	command.Flags().Int("count", 0, "Stop after this many matching messages (0 means unlimited)")
	command.Flags().Duration("timeout", 0, "Maximum observation duration, e.g. 30s (0 means until interrupted)")
	command.Flags().Bool("reconnect", true, "Reconnect on transient failures using Last-Event-ID")
	return command
}

func waitCommand() *cobra.Command {
	command := &cobra.Command{Use: "wait <type> <target>", Short: "Poll until a resource matches a JSON field value", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		path, _ := cmd.Flags().GetString("path")
		expected, _ := cmd.Flags().GetString("equals")
		timeout, _ := cmd.Flags().GetDuration("timeout")
		interval, _ := cmd.Flags().GetDuration("interval")
		if path == "" || !strings.HasPrefix(path, "/") {
			return fmt.Errorf("--path must be a JSON pointer such as /on/on")
		}
		if !json.Valid([]byte(expected)) {
			return fmt.Errorf("--equals must be a JSON value, e.g. true, 42 or a quoted string")
		}
		if timeout <= 0 || interval < 100*time.Millisecond {
			return fmt.Errorf("timeout must be positive and interval at least 100ms")
		}
		client, err := newClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		original := cmd.Context()
		cmd.SetContext(ctx)
		defer cmd.SetContext(original)
		raw, _ := cmd.Flags().GetBool("raw")
		id, err := resolveResourceTarget(cmd, client, args[0], args[1], raw)
		if err != nil {
			return err
		}
		for {
			resource, err := readResource(cmd, client, args[0], id)
			if err != nil {
				return err
			}
			value, found := jsonPointer(jsonValue(resource), path)
			if found && sameJSON(value, json.RawMessage(expected)) {
				return writeNativeResponse(cmd, hue.ResourceResponse{Data: []hue.Resource{resource}})
			}
			select {
			case <-ctx.Done():
				return &hue.Problem{Code: "timeout", Message: "resource did not reach the requested state", Details: map[string]any{"path": path, "expected": json.RawMessage(expected), "last_resource": resource}}
			case <-time.After(interval):
			}
		}
	}}
	command.Flags().String("path", "", "JSON pointer to observe")
	command.Flags().String("equals", "", "Expected JSON value")
	command.Flags().Duration("timeout", 30*time.Second, "Maximum wait")
	command.Flags().Duration("interval", time.Second, "Polling interval (minimum 100ms)")
	_ = command.MarkFlagRequired("path")
	_ = command.MarkFlagRequired("equals")
	return command
}

func jsonPointer(value json.RawMessage, pointer string) (json.RawMessage, bool) {
	for _, segment := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		segment = strings.ReplaceAll(strings.ReplaceAll(segment, "~1", "/"), "~0", "~")
		var object hue.Resource
		if json.Unmarshal(value, &object) == nil {
			var ok bool
			value, ok = object[segment]
			if !ok {
				return nil, false
			}
			continue
		}
		var array []json.RawMessage
		if json.Unmarshal(value, &array) != nil {
			return nil, false
		}
		index, err := strconv.Atoi(segment)
		if err != nil || index < 0 || index >= len(array) {
			return nil, false
		}
		value = array[index]
	}
	return value, true
}

func init() { rootCmd.AddCommand(watchCommand()) }
