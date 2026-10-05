package hue

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type EventMessage struct {
	ID   string          `json:"id,omitempty"`
	Data json.RawMessage `json:"events"`
}

func (c *Client) Events(ctx context.Context, lastID string, consume func(EventMessage) error) error {
	url := strings.TrimSuffix(c.baseURL, basePath) + "/eventstream/clip/v2"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	request.Header.Set("hue-application-key", c.appKey)
	request.Header.Set("Accept", "text/event-stream")
	if lastID != "" {
		request.Header.Set("Last-Event-ID", lastID)
	}
	client := *c.http
	client.Timeout = 0
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		return &APIError{StatusCode: response.StatusCode, Message: string(body)}
	}
	return readEvents(response.Body, lastID, consume)
}

func readEvents(reader io.Reader, lastID string, consume func(EventMessage) error) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 4<<20)
	var data []string
	size := 0
	emit := func() error {
		if len(data) == 0 {
			return nil
		}
		body := strings.Join(data, "\n")
		data = nil
		size = 0
		if !json.Valid([]byte(body)) {
			return fmt.Errorf("invalid JSON in Hue event stream")
		}
		return consume(EventMessage{ID: lastID, Data: json.RawMessage(body)})
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := emit(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "id":
			if !strings.ContainsRune(value, '\x00') {
				lastID = value
			}
		case "data":
			size += len(value)
			if size > 4<<20 {
				return fmt.Errorf("hue event exceeds 4 MiB")
			}
			data = append(data, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return io.EOF
}
