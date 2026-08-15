package config

import "testing"

func TestSaveAndLoadBridge(t *testing.T) {
	t.Setenv("HUEGO_CONFIG_DIR", t.TempDir())

	want := Bridge{IP: "192.168.1.10", AppKey: "test-app-key", ClientKey: "abc123"}
	if err := SaveBridge(want); err != nil {
		t.Fatalf("SaveBridge() error = %v", err)
	}

	got, err := LoadBridge()
	if err != nil {
		t.Fatalf("LoadBridge() error = %v", err)
	}
	if got != want {
		t.Fatalf("LoadBridge() = %+v, want %+v", got, want)
	}
}

func TestLoadBridgeEnvOverridesFile(t *testing.T) {
	t.Setenv("HUEGO_CONFIG_DIR", t.TempDir())
	if err := SaveBridge(Bridge{IP: "10.0.0.1", AppKey: "file-key"}); err != nil {
		t.Fatalf("SaveBridge() error = %v", err)
	}
	t.Setenv("HUEGO_BRIDGE_IP", "10.0.0.2")
	t.Setenv("HUEGO_APP_KEY", "env-key")

	got, err := LoadBridge()
	if err != nil {
		t.Fatalf("LoadBridge() error = %v", err)
	}
	if got.IP != "10.0.0.2" || got.AppKey != "env-key" {
		t.Fatalf("LoadBridge() = %+v, want env overrides applied", got)
	}
}

func TestLoadBridgeMissingReturnsError(t *testing.T) {
	t.Setenv("HUEGO_CONFIG_DIR", t.TempDir())

	if _, err := LoadBridge(); err == nil {
		t.Fatal("LoadBridge() error = nil, want error for missing bridge config")
	}
}
