package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

var apiMethods = []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete}

var apiCmd = &cobra.Command{
	Use:   "api <method> <path> [body]",
	Short: "Send a raw request to the bridge's CLIP v2 API",
	Long: "Send a raw request to the bridge's CLIP v2 API for anything without a dedicated\n" +
		"command. path is relative to /clip/v2/resource. body is JSON, or \"-\" to read it\n" +
		"from stdin. The response body is printed indented, or compact with --json.",
	Example: "  huego api get device\n" +
		"  huego api get light/<id>\n" +
		"  huego api put light/<id> '{\"on\":{\"on\":true}}'\n" +
		"  echo '{\"recall\":{\"action\":\"activate\"}}' | huego api put smart_scene/<id> -",
	Args: cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		method := strings.ToUpper(args[0])
		if !slices.Contains(apiMethods, method) {
			return usagef("invalid method %q: use one of %s", args[0], strings.ToLower(strings.Join(apiMethods, ", ")))
		}
		path := strings.TrimPrefix(strings.TrimPrefix(args[1], "/"), "clip/v2/resource")
		path = strings.TrimPrefix(path, "/")

		var body []byte
		if len(args) == 3 {
			var err error
			body, err = readAPIBody(cmd, args[2])
			if err != nil {
				return err
			}
		}

		client, err := newClient()
		if err != nil {
			return err
		}
		resp, err := client.Raw(cmd.Context(), method, path, body)
		if err != nil {
			return err
		}
		return writeRawJSON(cmd, resp)
	},
}

func readAPIBody(cmd *cobra.Command, arg string) ([]byte, error) {
	body := []byte(arg)
	if arg == "-" {
		var err error
		if body, err = io.ReadAll(cmd.InOrStdin()); err != nil {
			return nil, err
		}
	}
	if !json.Valid(body) {
		return nil, usagef("body is not valid JSON")
	}
	return body, nil
}

func writeRawJSON(cmd *cobra.Command, data []byte) error {
	var buf bytes.Buffer
	var err error
	if jsonEnabled(cmd) {
		err = json.Compact(&buf, data)
	} else {
		err = json.Indent(&buf, data, "", "  ")
	}
	if err != nil {
		_, writeErr := cmd.OutOrStdout().Write(data)
		return writeErr
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), buf.String())
	return err
}

func init() {
	rootCmd.AddCommand(apiCmd)
}
