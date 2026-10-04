package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

const maxPayloadBytes = 4 << 20

func addPayloadFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("data", "d", "", "CLIP request body as a JSON object")
	cmd.Flags().StringP("file", "f", "", "Read a JSON object from a file, or - for stdin")
	cmd.MarkFlagsMutuallyExclusive("data", "file")
	cmd.MarkFlagsOneRequired("data", "file")
}

func readPayload(cmd *cobra.Command) (json.RawMessage, error) {
	var reader io.Reader
	if cmd.Flags().Changed("data") {
		data, _ := cmd.Flags().GetString("data")
		reader = bytes.NewBufferString(data)
	} else {
		filename, _ := cmd.Flags().GetString("file")
		if filename == "-" {
			reader = cmd.InOrStdin()
		} else {
			file, err := os.Open(filename)
			if err != nil {
				return nil, fmt.Errorf("read payload: %w", err)
			}
			defer func() { _ = file.Close() }()
			reader = file
		}
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxPayloadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read payload: %w", err)
	}
	if len(data) > maxPayloadBytes {
		return nil, fmt.Errorf("JSON payload exceeds %d bytes", maxPayloadBytes)
	}
	// PowerShell's UTF-8 files may include a BOM; JSON itself does not permit one.
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, fmt.Errorf("invalid JSON payload: %w", err)
	}
	if object == nil {
		return nil, fmt.Errorf("JSON payload must be an object")
	}
	return json.RawMessage(data), nil
}
