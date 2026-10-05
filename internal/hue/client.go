package hue

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mathisarends/huego/internal/config"
)

const basePath = "/clip/v2/resource"

// Client talks to one Hue bridge's local CLIP v2 REST API. The bridge
// presents a self-signed certificate, so TLS verification is disabled.
type Client struct {
	bridge config.Bridge
	baseURL   string
	appKey    string
	http      *http.Client
	preview   func(string, string, json.RawMessage) error
	Resources *ResourceRegistry
	*Lights
	*Rooms
	*Zones
	*Scenes
}

func (c *Client) PreviewWrites(preview func(string, string, json.RawMessage) error) {
	c.preview = preview
}

func NewClient(bridge config.Bridge) *Client {
	c := &Client{
		bridge: bridge,
		baseURL: fmt.Sprintf("https://%s%s", bridge.IP, basePath),
		appKey:  bridge.AppKey,
		http: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // bridge cert is self-signed and unverifiable
			},
		},
	}
	c.Lights = newLights(c)
	c.Rooms = newRooms(c)
	c.Zones = newZones(c)
	c.Scenes = newScenes(c)
	c.Resources = &ResourceRegistry{client: c}
	return c
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	return c.request(ctx, http.MethodGet, path, nil, out)
}

func (c *Client) put(ctx context.Context, path string, body, out any) error {
	return c.request(ctx, http.MethodPut, path, body, out)
}

func (c *Client) request(ctx context.Context, method, path string, body, out any) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(b)
	}
	if c.preview != nil && method != http.MethodGet {
		var payload json.RawMessage
		if body != nil {
			var err error
			payload, err = json.Marshal(body)
			if err != nil {
				return err
			}
		}
		return c.preview(method, basePath+"/"+path, payload)
	}

	requestURL := c.baseURL
	if path != "" {
		requestURL += "/" + path
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("hue-application-key", c.appKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{StatusCode: resp.StatusCode, Message: string(respBody)}
	}

	if out == nil || len(respBody) == 0 {
		return nil
	}
	return json.Unmarshal(respBody, out)
}
