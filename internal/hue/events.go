package hue

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Event is one entry of the bridge's event stream. Data holds partial
// resources: only id, type, and the properties that changed.
type Event struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	CreationTime time.Time         `json:"creationtime"`
	Data         []json.RawMessage `json:"data"`
}

// EventResource identifies the resource one Event.Data entry belongs to.
type EventResource struct {
	ID    string             `json:"id"`
	Type  ResourceType       `json:"type"`
	Owner *ResourceReference `json:"owner,omitempty"`
	Raw   json.RawMessage    `json:"-"`
}

func (e Event) Resources() []EventResource {
	resources := make([]EventResource, 0, len(e.Data))
	for _, raw := range e.Data {
		var resource EventResource
		if err := json.Unmarshal(raw, &resource); err == nil {
			resource.Raw = raw
			resources = append(resources, resource)
		}
	}
	return resources
}

// ErrStreamClosed is returned when the bridge ends the event stream.
var ErrStreamClosed = errors.New("event stream closed by the bridge")

// Events connects to the bridge's event stream, calls connected once the
// bridge accepted the connection, and then handle for every event. It
// returns nil when ctx is cancelled and an error when the connection fails
// or ends.
func (c *Client) Events(ctx context.Context, connected func(), handle func(Event)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+c.host+"/eventstream/clip/v2", nil)
	if err != nil {
		return err
	}
	req.Header.Set("hue-application-key", c.appKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.stream.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return &APIError{StatusCode: resp.StatusCode, Message: string(body)}
	}
	if connected != nil {
		connected()
	}

	err = readEvents(resp.Body, handle)
	if ctx.Err() != nil {
		return nil
	}
	return err
}

// readEvents parses the server-sent events framing: "data:" lines up to a
// blank line form one message, which the bridge fills with a JSON array of
// events. Comment lines such as ": hi" and "id:" lines are ignored.
func readEvents(r io.Reader, handle func(Event)) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var data strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if data.Len() == 0 {
				continue
			}
			var events []Event
			if err := json.Unmarshal([]byte(data.String()), &events); err != nil {
				return fmt.Errorf("decode event: %w", err)
			}
			data.Reset()
			for _, event := range events {
				handle(event)
			}
			continue
		}
		if value, ok := strings.CutPrefix(line, "data:"); ok {
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimPrefix(value, " "))
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return ErrStreamClosed
}

// EventWatch receives the lifecycle of WatchEvents. Every callback is
// optional and called from the watching goroutine.
type EventWatch struct {
	Connected    func()
	Event        func(Event)
	Disconnected func(err error, retryIn time.Duration)
}

const (
	minReconnectDelay = time.Second
	maxReconnectDelay = 30 * time.Second
)

// WatchEvents keeps the event stream open, reconnecting with exponential
// backoff after network errors or bridge restarts. It returns nil when ctx
// is cancelled, and returns early only when the bridge rejects the app key,
// since retrying cannot fix that.
func (c *Client) WatchEvents(ctx context.Context, watch EventWatch) error {
	delay := minReconnectDelay
	for {
		connected := func() {
			delay = minReconnectDelay
			if watch.Connected != nil {
				watch.Connected()
			}
		}
		handle := func(event Event) {
			if watch.Event != nil {
				watch.Event(event)
			}
		}
		err := c.Events(ctx, connected, handle)
		if ctx.Err() != nil {
			return nil
		}
		if IsAuthError(err) {
			return err
		}
		if watch.Disconnected != nil {
			watch.Disconnected(err, delay)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(delay):
		}
		delay = min(delay*2, maxReconnectDelay)
	}
}

// IsAuthError reports whether the bridge rejected the application key.
func IsAuthError(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden)
}
