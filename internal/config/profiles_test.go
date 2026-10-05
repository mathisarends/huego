package config

import (
	"testing"
)

func TestProfilesIsolateCredentialsAndReplace(t *testing.T) {
	t.Setenv("HUEGO_CONFIG_DIR", t.TempDir())
	t.Setenv("HUEGO_APP_KEY", "wrong-environment-key")
	if err := SaveProfile("office", Bridge{IP: "192.0.2.1", AppKey: "profile-key"}); err != nil {
		t.Fatal(err)
	}
	if err := SaveProfile("office", Bridge{IP: "192.0.2.2", AppKey: "new-key"}); err != nil {
		t.Fatal(err)
	}
	profile, err := LoadProfile("office")
	if err != nil || profile.IP != "192.0.2.2" || profile.AppKey != "new-key" {
		t.Fatalf("profile=%v err=%v", profile, err)
	}
	profiles, err := Profiles()
	if err != nil || len(profiles) != 1 || profiles[0].Name != "office" {
		t.Fatal(profiles, err)
	}
	t.Setenv("HUEGO_PROFILE", "office")
	profile, err = LoadBridge()
	if err != nil || profile.AppKey != "new-key" {
		t.Fatal(profile, err)
	}
	for _, name := range []string{"../escape", "a/b", "", "a:b"} {
		if err := SaveProfile(name, profile); err == nil {
			t.Fatalf("accepted name %q", name)
		}
	}
}
