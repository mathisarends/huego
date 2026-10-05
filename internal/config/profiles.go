package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var profilePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

func profilePath(name string) (string, error) {
	if !profilePattern.MatchString(name) {
		return "", fmt.Errorf("bridge profile must contain 1-64 letters, digits, hyphens or underscores")
	}
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "bridges", name+".json"), nil
}

func LoadProfile(name string) (Bridge, error) {
	var bridge Bridge
	path, err := profilePath(name)
	if err != nil {
		return bridge, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return bridge, fmt.Errorf("load bridge profile %q: %w", name, err)
	}
	if err := json.Unmarshal(data, &bridge); err != nil {
		return bridge, err
	}
	if bridge.IP == "" || bridge.AppKey == "" {
		return bridge, ErrNotConfigured
	}
	return bridge, nil
}

func SaveProfile(name string, bridge Bridge) error {
	if bridge.IP == "" || bridge.AppKey == "" {
		return ErrNotConfigured
	}
	path, err := profilePath(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(bridge, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".bridge-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(file.Name()) }()
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

type Profile struct {
	Name string `json:"name"`
	IP   string `json:"bridge_ip"`
}

func Profiles() ([]Profile, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(dir, "bridges"))
	if os.IsNotExist(err) {
		return []Profile{}, nil
	}
	if err != nil {
		return nil, err
	}
	profiles := []Profile{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		bridge, err := LoadProfile(name)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, Profile{Name: name, IP: bridge.IP})
	}
	return profiles, nil
}
