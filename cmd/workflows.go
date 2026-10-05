package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"reflect"
	"strings"

	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

type resourceChange struct {
	Kind       string
	ID         string
	Method     string
	Body       hue.Resource
	GuardField string
	Before     json.RawMessage
}

func jsonValue(value any) json.RawMessage { data, _ := json.Marshal(value); return data }

func sameJSON(a, b json.RawMessage) bool {
	var x, y any
	left, right := json.NewDecoder(bytes.NewReader(a)), json.NewDecoder(bytes.NewReader(b))
	left.UseNumber()
	right.UseNumber()
	if left.Decode(&x) != nil || right.Decode(&y) != nil {
		return string(a) == string(b)
	}
	return reflect.DeepEqual(exactJSONNumbers(x), exactJSONNumbers(y))
}

func exactJSONNumbers(value any) any {
	switch node := value.(type) {
	case json.Number:
		number, ok := new(big.Rat).SetString(string(node))
		if ok {
			return number
		}
	case map[string]any:
		for key, child := range node {
			node[key] = exactJSONNumbers(child)
		}
	case []any:
		for index, child := range node {
			node[index] = exactJSONNumbers(child)
		}
	}
	return value
}

func executeChanges(cmd *cobra.Command, client *hue.Client, changes []resourceChange) error {
	plans := make([]requestPlan, 0, len(changes))
	for _, change := range changes {
		body := jsonValue(change.Body)
		if err := hue.ValidateRequest(change.Kind, change.Method, body); err != nil {
			return err
		}
		validation := &planValidation{Schema: "valid", References: "not_checked", Capabilities: "not_checked"}
		if check, _ := cmd.Flags().GetBool("check"); check {
			if err := checkRequest(cmd, client, change.Kind, change.ID, body); err != nil {
				return err
			}
			validation.References, validation.Capabilities = "checked_local_references", "checked_supported_fields"
		}
		path, err := hue.ResourcePath(change.Kind, change.Method, change.ID, false)
		if err != nil {
			return err
		}
		plans = append(plans, requestPlan{Method: change.Method, Path: "/clip/v2/resource/" + path, Body: body, Validation: validation})
	}
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if dryRun {
		if len(plans) == 1 {
			return writeStructured(cmd, plans[0])
		}
		return writeStructured(cmd, map[string]any{"steps": plans, "atomic": false, "changed": len(plans) > 0})
	}
	results := make([]hue.ResourceResponse, 0, len(changes))
	for index, change := range changes {
		var err error
		if change.GuardField != "" {
			var current hue.Resource
			current, err = readResource(cmd, client, change.Kind, change.ID)
			if err == nil && !sameJSON(current[change.GuardField], change.Before) {
				err = &hue.Problem{Code: "concurrent_change", Message: "resource changed while preparing the update; rerun against the current state"}
			}
		}
		var response hue.ResourceResponse
		if err == nil {
			var ns *hue.NativeNamespace
			ns, err = client.Resources.Of(change.Kind, false)
			if err == nil {
				response, err = ns.Write(cmd.Context(), change.Method, change.ID, plans[index].Body)
			}
			if err == nil {
				err = response.Err()
			}
		}
		if err != nil {
			return &hue.Problem{Code: "workflow_failed", Message: err.Error(), Details: map[string]any{"cause": commandProblem(err), "completed": results, "failed_step": index, "response": response, "remaining": plans[index:], "atomic": false}}
		}
		results = append(results, response)
	}
	if len(results) == 1 {
		return writeNativeResponse(cmd, results[0])
	}
	return writeStructured(cmd, map[string]any{"changed": len(results) > 0, "results": results, "atomic": false})
}

func groupLightIDs(group hue.Resource, lights []hue.Resource) map[string]bool {
	members := map[string]bool{}
	for _, child := range decodeField[[]hue.ResourceReference](group, "children") {
		for _, light := range lights {
			owner := decodeField[hue.ResourceReference](light, "owner")
			if (child.RType == "light" && child.RID == light.GetID()) || (child.RType == "device" && child.RID == owner.RID) {
				members[light.GetID()] = true
			}
		}
	}
	return members
}

func membershipCommand(kind string, remove bool) *cobra.Command {
	action := "add"
	if remove {
		action = "remove"
	}
	command := &cobra.Command{Use: action + " <group> <light>...", Short: action + " group members while preserving other members", Args: cobra.MinimumNArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		id, err := resolveResourceTarget(cmd, client, kind, args[0], false)
		if err != nil {
			return err
		}
		group, err := readResource(cmd, client, kind, id)
		if err != nil {
			return err
		}
		children := decodeField[[]hue.ResourceReference](group, "children")
		devices, _ := cmd.Flags().GetBool("devices")
		for _, target := range args[1:] {
			memberKind := "light"
			if devices {
				memberKind = "device"
			}
			memberID, err := resolveResourceTarget(cmd, client, memberKind, target, false)
			if err != nil {
				return err
			}
			member, err := readResource(cmd, client, memberKind, memberID)
			if err != nil {
				return err
			}
			ref := hue.ResourceReference{RID: memberID, RType: hue.ResourceType(memberKind)}
			if kind == "room" && !devices {
				ref = decodeField[hue.ResourceReference](member, "owner")
				if ref.RType != "device" || ref.RID == "" {
					return fmt.Errorf("light %q has no device owner", target)
				}
			}
			if kind == "zone" && devices {
				return fmt.Errorf("zones use light services; omit --devices")
			}
			found := false
			next := make([]hue.ResourceReference, 0, len(children)+1)
			for _, child := range children {
				if child == ref {
					found = true
					if remove {
						continue
					}
				}
				next = append(next, child)
			}
			if !remove && !found {
				next = append(next, ref)
			}
			children = next
		}
		body := jsonValue(children)
		changes := []resourceChange{}
		if !sameJSON(group["children"], body) {
			changes = append(changes, resourceChange{Kind: kind, ID: id, Method: http.MethodPut, Body: hue.Resource{"children": body}, GuardField: "children", Before: group["children"]})
		}
		return executeChanges(cmd, client, changes)
	}}
	command.Flags().Bool("devices", false, "Resolve room members as devices instead of lights (for accessories)")
	command.Flags().Bool("dry-run", false, "Read current members and preview the change")
	return command
}

func roomMoveCommand() *cobra.Command {
	command := &cobra.Command{Use: "move <light> <destination-room>", Short: "Move a light's device between rooms; report partial progress on failure", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		id, err := resolveResourceTarget(cmd, client, "light", args[0], false)
		if err != nil {
			return err
		}
		light, err := readResource(cmd, client, "light", id)
		if err != nil {
			return err
		}
		owner := decodeField[hue.ResourceReference](light, "owner")
		if owner.RType != "device" || owner.RID == "" {
			return fmt.Errorf("light has no device owner")
		}
		rooms, err := readResources(cmd, client, "room")
		if err != nil {
			return err
		}
		destination, err := findTarget(rooms, "room", args[1])
		if err != nil {
			return err
		}
		changes := []resourceChange{}
		var addition *resourceChange
		for _, room := range rooms {
			children := decodeField[[]hue.ResourceReference](room, "children")
			next := make([]hue.ResourceReference, 0, len(children)+1)
			found := false
			for _, child := range children {
				if child == owner {
					found = true
					if room.GetID() != destination {
						continue
					}
				}
				next = append(next, child)
			}
			if room.GetID() == destination && !found {
				next = append(next, owner)
			}
			if !sameJSON(room["children"], jsonValue(next)) {
				change := resourceChange{Kind: "room", ID: room.GetID(), Method: http.MethodPut, Body: hue.Resource{"children": jsonValue(next)}, GuardField: "children", Before: room["children"]}
				if room.GetID() == destination {
					addition = &change
				} else {
					changes = append(changes, change)
				}
			}
		}
		if addition != nil {
			changes = append(changes, *addition)
		}
		return executeChanges(cmd, client, changes)
	}}
	command.Flags().Bool("dry-run", false, "Preview all steps without writing")
	return command
}

func ensureCommand(kind string) *cobra.Command {
	command := &cobra.Command{Use: "ensure <name>", Short: "Create or update a named resource; reuse its ID on subsequent runs", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		body, err := readPayload(cmd)
		if err != nil {
			return err
		}
		body, err = applySceneScope(cmd, kind, body)
		if err != nil {
			return err
		}
		var payload hue.Resource
		if err = json.Unmarshal(body, &payload); err != nil {
			return err
		}
		metadata := decodeField[hue.Resource](payload, "metadata")
		if metadata == nil {
			metadata = hue.Resource{}
		}
		metadata["name"] = jsonValue(strings.TrimSpace(args[0]))
		payload["metadata"] = jsonValue(metadata)
		client, err := newClient()
		if err != nil {
			return err
		}
		return ensureResource(cmd, client, kind, args[0], payload)
	}}
	addPayloadFlags(command)
	command.Flags().Bool("dry-run", false, "Read existing state and preview create/update")
	return command
}

func ensureResource(cmd *cobra.Command, client *hue.Client, kind, name string, payload hue.Resource) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("name must not be empty")
	}
	if err := hue.ValidateRequest(kind, http.MethodPost, jsonValue(payload)); err != nil {
		return err
	}
	resources, err := readResources(cmd, client, kind)
	if err != nil {
		return err
	}
	var existing []hue.Resource
	group := decodeField[hue.ResourceReference](payload, "group")
	for _, resource := range resources {
		if strings.EqualFold(strings.TrimSpace(resource.GetName()), strings.TrimSpace(name)) {
			if (kind == "scene" || kind == "smart_scene") && decodeField[hue.ResourceReference](resource, "group") != group {
				continue
			}
			existing = append(existing, resource)
		}
	}
	if len(existing) > 1 {
		_, err := findTarget(existing, kind, name)
		return err
	}
	method, id := http.MethodPost, ""
	if len(existing) == 1 {
		if kind == "behavior_instance" && !sameJSON(existing[0]["script_id"], payload["script_id"]) {
			return &hue.Problem{Code: "immutable_field", Message: "existing automation uses a different script; choose another name or explicitly replace it"}
		}
		method, id = http.MethodPut, existing[0].GetID()
		if kind == "scene" || kind == "smart_scene" {
			delete(payload, "group")
		}
		if kind == "behavior_instance" {
			delete(payload, "script_id")
		}
		if matchesSubset(existing[0], payload) {
			return writeStructured(cmd, map[string]any{"changed": false, "errors": []any{}, "data": []hue.ResourceReference{{RID: id, RType: hue.ResourceType(kind)}}})
		}
	}
	return executeChanges(cmd, client, []resourceChange{{Kind: kind, ID: id, Method: method, Body: payload}})
}

func matchesSubset(current, want hue.Resource) bool {
	for key, value := range want {
		var child hue.Resource
		if json.Unmarshal(value, &child) == nil && child != nil {
			var have hue.Resource
			if json.Unmarshal(current[key], &have) != nil || !matchesSubset(have, child) {
				return false
			}
		} else if !sameJSON(current[key], value) {
			return false
		}
	}
	return true
}

func init() {
	roomCmd.AddCommand(membershipCommand("room", false), membershipCommand("room", true), roomMoveCommand())
	zoneCmd.AddCommand(membershipCommand("zone", false), membershipCommand("zone", true))
}
