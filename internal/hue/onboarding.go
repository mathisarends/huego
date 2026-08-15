package hue

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type DiscoveredBridge struct {
	ID                string `json:"id"`
	InternalIPAddress string `json:"internalipaddress"`
}

// DiscoverBridges asks Philips's cloud discovery service for bridges on the
// local network, same as hueify's discover_bridges().
func DiscoverBridges(ctx context.Context) ([]DiscoveredBridge, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://discovery.meethue.com/", nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var bridges []DiscoveredBridge
	if err := json.NewDecoder(resp.Body).Decode(&bridges); err != nil {
		return nil, err
	}
	if len(bridges) == 0 {
		return nil, fmt.Errorf("no Hue bridge found on the network")
	}
	return bridges, nil
}

type RegisteredApp struct {
	AppKey    string
	ClientKey string
}

const (
	registrationTimeout = 60 * time.Second
	defaultDeviceType   = "huego#setup"
)

// registrationPollInterval is a var so tests can shorten it.
var registrationPollInterval = 2 * time.Second

// RegisterAppKey polls the bridge until its link button is pressed, then
// exchanges that press for an application key (and a client key for
// entertainment streaming), same as hueify's register_app_key().
func RegisterAppKey(ctx context.Context, bridgeIP string) (RegisteredApp, error) {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // bridge cert is self-signed and unverifiable
		},
	}

	payload, err := json.Marshal(map[string]any{
		"devicetype":        defaultDeviceType,
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
			return RegisteredApp{}, err
		}

		var results []struct {
			Success *struct {
				Username  string `json:"username"`
				ClientKey string `json:"clientkey"`
			} `json:"success"`
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

		select {
		case <-ctx.Done():
			return RegisteredApp{}, ctx.Err()
		case <-time.After(registrationPollInterval):
		}
	}

	return RegisteredApp{}, fmt.Errorf("link button was not pressed within %s", registrationTimeout)
}
