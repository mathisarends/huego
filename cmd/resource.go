package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

type requestPlan struct {
	Method string          `json:"method"`
	Path   string          `json:"path"`
	Body   json.RawMessage `json:"body,omitempty"`
}

func writeStructured(cmd *cobra.Command, value any) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetEscapeHTML(false)
	compact, _ := cmd.Flags().GetBool("json")
	if !compact {
		encoder.SetIndent("", "  ")
	}
	return encoder.Encode(value)
}

func writeNativeResponse(cmd *cobra.Command, response hue.ResourceResponse) error {
	if response.Data == nil {
		response.Data = []hue.Resource{}
	}
	if response.Errors == nil {
		response.Errors = []json.RawMessage{}
	}
	if err := writeStructured(cmd, response); err != nil {
		return err
	}
	return response.Err()
}

func newResourceCommand() *cobra.Command {
	parent := &cobra.Command{
		Use: "resource", Aliases: []string{"resources"},
		Short: "Inspect and configure every CLIP v2 resource with native JSON",
	}
	parent.PersistentFlags().Bool("raw", false, "Bypass the pinned operation catalog for firmware-specific endpoints")
	types := &cobra.Command{
		Use: "types", Short: "Show documented resource types and HTTP operations without contacting a bridge", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			catalog := hue.Catalog()
			for i := range catalog.Resources {
				catalog.Resources[i].RequestSchemas = nil
			}
			return writeStructured(cmd, catalog)
		},
	}
	schema := &cobra.Command{
		Use: "schema <type>", Short: "Show offline request schemas and supported operations from OpenHue", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			capability, err := hue.Capability(args[0])
			if err != nil {
				return err
			}
			catalog := hue.Catalog()
			return writeStructured(cmd, struct {
				Source   string `json:"source"`
				Revision string `json:"revision"`
				hue.ResourceCapability
			}{catalog.Source, catalog.Revision, capability})
		},
	}
	list := &cobra.Command{
		Use: "list [type]", Short: "Read the complete resource inventory, or one resource type", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, _ := cmd.Flags().GetBool("raw")
			if len(args) != 0 {
				if _, err := hue.ResourcePath(args[0], http.MethodGet, "", raw); err != nil {
					return err
				}
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			var response hue.ResourceResponse
			if len(args) == 0 {
				response, err = client.Resources.List(cmd.Context())
			} else {
				namespace, nsErr := client.Resources.Of(args[0], raw)
				if nsErr != nil {
					return nsErr
				}
				response, err = namespace.List(cmd.Context())
			}
			if err != nil {
				return err
			}
			return writeNativeResponse(cmd, response)
		},
	}
	parent.AddCommand(types, schema, list, newNativeGetCommand(""))
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		parent.AddCommand(newNativeWriteCommand("", method))
	}
	return parent
}

func newNativeGetCommand(kind string) *cobra.Command {
	use := "get <target>"
	count := 1
	if kind == "" {
		use = "get <type> <target>"
		count++
	}
	return &cobra.Command{
		Use: use, Short: "Read a complete resource by UUID or unique exact name", Args: cobra.ExactArgs(count),
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceType, target := kind, args[0]
			if resourceType == "" {
				resourceType, target = args[0], args[1]
			}
			raw, _ := cmd.Flags().GetBool("raw")
			if _, err := hue.ResourcePath(resourceType, http.MethodGet, "target", raw); err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			namespace, err := client.Resources.Of(resourceType, raw)
			if err != nil {
				return err
			}
			id, err := namespace.ResolveTarget(cmd.Context(), target)
			if err != nil {
				return err
			}
			response, err := namespace.Get(cmd.Context(), id)
			if err != nil {
				return err
			}
			if err := writeNativeResponse(cmd, response); err != nil {
				return err
			}
			if len(response.Data) == 0 {
				return fmt.Errorf("no %s with ID %s", resourceType, id)
			}
			return nil
		},
	}
}

func newNativeWriteCommand(kind, method string) *cobra.Command {
	action := map[string]string{http.MethodPost: "create", http.MethodPut: "update", http.MethodDelete: "delete"}[method]
	use := action
	count := 0
	if kind == "" {
		use += " <type>"
		count++
	}
	if method != http.MethodPost {
		use += " <target>"
		count++
	}
	command := &cobra.Command{
		Use: use, Short: action + " a native CLIP resource", Args: cobra.ExactArgs(count),
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceType := kind
			if resourceType == "" {
				resourceType, args = args[0], args[1:]
			}
			var body json.RawMessage
			if method != http.MethodDelete {
				var err error
				body, err = readPayload(cmd)
				if err != nil {
					return err
				}
			}
			target := ""
			if len(args) != 0 {
				target = args[0]
			}
			return runNativeWrite(cmd, resourceType, method, target, body)
		},
	}
	if method != http.MethodDelete {
		addPayloadFlags(command)
	}
	command.Flags().Bool("dry-run", false, "Print the method, path and payload without writing; names may require a bridge read")
	return command
}

func runNativeWrite(cmd *cobra.Command, kind, method, target string, body json.RawMessage) error {
	raw, _ := cmd.Flags().GetBool("raw")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	placeholder := ""
	if method != http.MethodPost {
		placeholder = "target"
		if strings.TrimSpace(target) == "" {
			return fmt.Errorf("target must be a UUID or a non-empty name")
		}
	}
	if _, err := hue.ResourcePath(kind, method, placeholder, raw); err != nil {
		return err
	}
	var client *hue.Client
	var err error
	id := ""
	if method != http.MethodPost {
		if hue.IsUUID(target) {
			id = strings.ToLower(target)
		} else {
			client, err = newClient()
			if err != nil {
				return err
			}
			namespace, nsErr := client.Resources.Of(kind, raw)
			if nsErr != nil {
				return nsErr
			}
			id, err = namespace.ResolveTarget(cmd.Context(), target)
			if err != nil {
				return err
			}
		}
	}
	path, err := hue.ResourcePath(kind, method, id, raw)
	if err != nil {
		return err
	}
	if dryRun {
		return writeStructured(cmd, requestPlan{Method: method, Path: "/clip/v2/resource/" + path, Body: body})
	}
	if client == nil {
		client, err = newClient()
		if err != nil {
			return err
		}
	}
	namespace, err := client.Resources.Of(kind, raw)
	if err != nil {
		return err
	}
	response, err := namespace.Write(cmd.Context(), method, id, body)
	if err != nil {
		return err
	}
	return writeNativeResponse(cmd, response)
}

func init() {
	rootCmd.AddCommand(newResourceCommand())
}
