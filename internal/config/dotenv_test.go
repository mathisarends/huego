package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeDotEnv(t *testing.T, contents string) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(contents), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
}

func TestLoadDotEnvSetsUnsetVars(t *testing.T) {
	writeDotEnv(t, "HUEGO_BRIDGE_IP=192.168.1.20\n")
	_ = os.Unsetenv("HUEGO_BRIDGE_IP")

	loadDotEnv()

	if got := os.Getenv("HUEGO_BRIDGE_IP"); got != "192.168.1.20" {
		t.Fatalf("HUEGO_BRIDGE_IP = %q, want 192.168.1.20", got)
	}
}

func TestLoadDotEnvDoesNotOverrideExisting(t *testing.T) {
	writeDotEnv(t, "HUEGO_APP_KEY=from-dotenv\n")
	t.Setenv("HUEGO_APP_KEY", "from-environment")

	loadDotEnv()

	if got := os.Getenv("HUEGO_APP_KEY"); got != "from-environment" {
		t.Fatalf("HUEGO_APP_KEY = %q, want from-environment to win over .env", got)
	}
}

func TestLoadDotEnvSkipsCommentsAndBlankLines(t *testing.T) {
	writeDotEnv(t, "# a comment\n\nHUEGO_CLIENT_KEY=\"quoted-value\"\n")
	_ = os.Unsetenv("HUEGO_CLIENT_KEY")

	loadDotEnv()

	if got := os.Getenv("HUEGO_CLIENT_KEY"); got != "quoted-value" {
		t.Fatalf("HUEGO_CLIENT_KEY = %q, want quoted-value with quotes trimmed", got)
	}
}

func TestLoadDotEnvIgnoresMissingFile(t *testing.T) {
	t.Chdir(t.TempDir())
	loadDotEnv() // must not panic when .env doesn't exist
}
