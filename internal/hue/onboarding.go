package hue

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type DiscoveredBridge struct {
	ID                string `json:"id"`
	InternalIPAddress string `json:"internalipaddress"`
}

// discoveryURL is a var so tests can point discovery at a fake server.
var discoveryURL = "https://discovery.meethue.com/"

// DiscoverBridges asks Philips's cloud discovery service for bridges on the
// local network, same as hueify's discover_bridges().
func DiscoverBridges(ctx context.Context) ([]DiscoveredBridge, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("bridge discovery failed: %w; pass the bridge address with --ip instead", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		// The discovery service rate-limits clients (HTTP 429) after a few
		// requests per minute, which repeated setup attempts easily hit.
		return nil, fmt.Errorf("bridge discovery failed with HTTP %d; pass the bridge address with --ip instead", resp.StatusCode)
	}

	var bridges []DiscoveredBridge
	if err := json.NewDecoder(resp.Body).Decode(&bridges); err != nil {
		return nil, err
	}
	if len(bridges) == 0 {
		return nil, fmt.Errorf("no Hue bridge found on the network; pass the bridge address with --ip if you know it")
	}
	return bridges, nil
}

type RegisteredApp struct {
	AppKey    string
	ClientKey string
}

const (
	registrationTimeout  = 60 * time.Second
	linkButtonNotPressed = 101
)

// registrationPollInterval is a var so tests can shorten it.
var registrationPollInterval = 2 * time.Second

// RegisterAppKey polls the bridge until its link button is pressed, then
// exchanges that press for an application key (and a client key for
// entertainment streaming), same as hueify's register_app_key().
func RegisterAppKey(ctx context.Context, bridgeIP string) (RegisteredApp, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // bridge cert is self-signed and unverifiable
		},
	}

	payload, err := json.Marshal(map[string]any{
		"devicetype":        deviceType(),
		"generateclientkey": true,
	})
	if err != nil {
		return RegisteredApp{}, err
	}

	deadline := time.Now().Add(registrationTimeout)
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+bridgeIP+"/api", bytes.NewReader(payload))
		if err != nil {
			return RegisteredApp{}, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return RegisteredApp{}, fmt.Errorf("cannot reach the Hue bridge at %s: %w", bridgeIP, err)
		}

		var results []struct {
			Success *struct {
				Username  string `json:"username"`
				ClientKey string `json:"clientkey"`
			} `json:"success"`
			Error *struct {
				Type        int    `json:"type"`
				Description string `json:"description"`
			} `json:"error"`
		}
		decodeErr := json.NewDecoder(resp.Body).Decode(&results)
		_ = resp.Body.Close()
		if decodeErr != nil {
			return RegisteredApp{}, decodeErr
		}

		if len(results) > 0 && results[0].Success != nil {
			return RegisteredApp{
				AppKey:    results[0].Success.Username,
				ClientKey: results[0].Success.ClientKey,
			}, nil
		}
		if len(results) > 0 && results[0].Error != nil && results[0].Error.Type != linkButtonNotPressed {
			return RegisteredApp{}, fmt.Errorf("bridge refused registration: %s", results[0].Error.Description)
		}

		select {
		case <-ctx.Done():
			return RegisteredApp{}, ctx.Err()
		case <-time.After(registrationPollInterval):
		}
	}

	return RegisteredApp{}, fmt.Errorf("link button was not pressed within %s", registrationTimeout)
}

// deviceType names the registration "huego#<hostname>" so the bridge's app
// list in the Hue app shows which machine a key belongs to. The bridge
// limits the part after "#" to 19 characters.
func deviceType() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "cli"
	}
	if len(host) > 19 {
		host = host[:19]
	}
	return "huego#" + host
}
