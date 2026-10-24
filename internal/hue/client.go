package hue

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mathisarends/huego/internal/config"
)

const basePath = "/clip/v2/resource"

// Client talks to one Hue bridge's local CLIP v2 REST API. The bridge
// presents a self-signed certificate, so TLS verification is disabled.
type Client struct {
	host    string
	baseURL string
	appKey  string
	http    *http.Client
	stream  *http.Client

	Lights  *Lights
	Rooms   *Rooms
	Zones   *Zones
	Scenes  *Scenes
	Home    *Home
	Devices *Devices
}

func NewClient(bridge config.Bridge) *Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // bridge cert is self-signed and unverifiable
	}
	c := &Client{
		host:    bridge.IP,
		baseURL: fmt.Sprintf("https://%s%s", bridge.IP, basePath),
		appKey:  bridge.AppKey,
		http:    &http.Client{Timeout: 10 * time.Second, Transport: transport},
		// The event stream is a long-lived response, so it must not share the
		// request timeout; dead connections are detected by TCP keep-alives.
		stream: &http.Client{Transport: transport},
	}
	c.Lights = newLights(c)
	c.Rooms = newRooms(c)
	c.Zones = newZones(c)
	c.Scenes = newScenes(c)
	c.Home = newHome(c)
	c.Devices = newDevices(c)
	return c
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	return c.request(ctx, http.MethodGet, path, nil, out)
}

func (c *Client) put(ctx context.Context, path string, body, out any) error {
	return c.request(ctx, http.MethodPut, path, body, out)
}

// unreachable rewords a transport error without the request URL, which
// repeats the bridge address and adds nothing for the reader.
func (c *Client) unreachable(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}
	return fmt.Errorf("cannot reach the Hue bridge at %s: %w", c.host, err)
}

func (c *Client) request(ctx context.Context, method, path string, body, out any) error {
	var payload []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = b
	}

	respBody, err := c.Raw(ctx, method, path, payload)
	if err != nil {
		return err
	}
	if out == nil || len(respBody) == 0 {
		return nil
	}
	return json.Unmarshal(respBody, out)
}

// Raw sends a request to the CLIP v2 resource API and returns the response
// body unparsed. path is relative to /clip/v2/resource, e.g. "light" or
// "scene/<id>", and empty for every resource; body, when not nil, is sent
// as JSON.
func (c *Client) Raw(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	endpoint := c.baseURL
	if path = strings.Trim(path, "/"); path != "" {
		endpoint += "/" + path
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("hue-application-key", c.appKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, c.unreachable(err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return respBody, &APIError{StatusCode: resp.StatusCode, Message: string(respBody)}
	}
	return respBody, nil
}
