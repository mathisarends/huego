package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

var requestExamples = map[string]map[string]json.RawMessage{
	"room":          {"POST": json.RawMessage(`{"metadata":{"name":"Office","archetype":"office"},"children":[{"rid":"00000000-0000-0000-0000-000000000001","rtype":"device"}]}`)},
	"zone":          {"POST": json.RawMessage(`{"metadata":{"name":"Desk","archetype":"office"},"children":[{"rid":"00000000-0000-0000-0000-000000000001","rtype":"light"}]}`)},
	"scene":         {"POST": json.RawMessage(`{"metadata":{"name":"Focus"},"group":{"rid":"00000000-0000-0000-0000-000000000002","rtype":"room"},"actions":[{"target":{"rid":"00000000-0000-0000-0000-000000000001","rtype":"light"},"action":{"on":{"on":true},"dimming":{"brightness":75}}}]}`)},
	"smart_scene":   {"POST": json.RawMessage(`{"metadata":{"name":"Working day"},"group":{"rid":"00000000-0000-0000-0000-000000000002","rtype":"room"},"week_timeslots":[{"recurrence":["monday","tuesday","wednesday","thursday","friday"],"timeslots":[{"start_time":{"kind":"time","time":{"hour":8,"minute":0,"second":0}},"target":{"rid":"00000000-0000-0000-0000-000000000001","rtype":"scene"}}]}]}`)},
	"light":         {"PUT": json.RawMessage(`{"on":{"on":true},"dimming":{"brightness":50}}`)},
	"grouped_light": {"PUT": json.RawMessage(`{"on":{"on":true},"dimming":{"brightness":50}}`)},
}

func exampleCommand() *cobra.Command {
	command := &cobra.Command{Use: "example <type>", Short: "Print editable request JSON with placeholder UUIDs; script configuration needs its own schema", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		method, _ := cmd.Flags().GetString("method")
		method = strings.ToUpper(method)
		capability, err := hue.Capability(args[0])
		if err != nil {
			return err
		}
		example := capability.RequestExamples[method]
		if len(example) == 0 {
			example = requestExamples[args[0]][method]
		}
		if len(example) == 0 {
			return fmt.Errorf("no curated %s example for %s; use resource schema", method, args[0])
		}
		if err := hue.ValidateRequest(args[0], method, example); err != nil {
			return err
		}
		return writeStructured(cmd, example)
	}}
	command.Flags().String("method", "POST", "HTTP method: POST or PUT")
	return command
}
