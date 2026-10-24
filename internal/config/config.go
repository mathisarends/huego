package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const (
	dirName    = "huego"
	bridgeFile = "bridge.json"
)

// ErrNotConfigured means neither a saved bridge nor the environment
// provides a bridge IP and app key.
var ErrNotConfigured = errors.New("no Hue bridge configured: run `huego setup` or set HUEGO_BRIDGE_IP and HUEGO_APP_KEY")

type Bridge struct {
	IP        string `json:"bridge_ip"`
	AppKey    string `json:"app_key"`
	ClientKey string `json:"client_key,omitempty"`
}

// Dir returns the directory huego stores its bridge config in.
// HUEGO_CONFIG_DIR overrides the OS default, which is mainly useful for tests.
func Dir() (string, error) {
	if v := os.Getenv("HUEGO_CONFIG_DIR"); v != "" {
		if err := os.MkdirAll(v, 0o700); err != nil {
			return "", err
		}
		return v, nil
	}

	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, dirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// LoadBridge reads bridge config from the stored file, then applies any
// HUEGO_BRIDGE_IP, HUEGO_APP_KEY, HUEGO_CLIENT_KEY env vars on top.
func LoadBridge() (Bridge, error) {
	loadDotEnv()
	var bridge Bridge

	dir, err := Dir()
	if err != nil {
		return bridge, err
	}
	if data, err := os.ReadFile(filepath.Join(dir, bridgeFile)); err == nil {
		_ = json.Unmarshal(data, &bridge)
	}

	if v := os.Getenv("HUEGO_BRIDGE_IP"); v != "" {
		bridge.IP = v
	}
	if v := os.Getenv("HUEGO_APP_KEY"); v != "" {
		bridge.AppKey = v
	}
	if v := os.Getenv("HUEGO_CLIENT_KEY"); v != "" {
		bridge.ClientKey = v
	}

	if bridge.IP == "" || bridge.AppKey == "" {
		return bridge, ErrNotConfigured
	}
	return bridge, nil
}

func SaveBridge(bridge Bridge) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(bridge, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, bridgeFile), data, 0o600)
}
