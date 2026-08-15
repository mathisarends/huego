package cmd

import "testing"

func TestParseFloatValid(t *testing.T) {
	f, err := parseFloat("42.5", "brightness")
	if err != nil {
		t.Fatalf("parseFloat() error = %v", err)
	}
	if f != 42.5 {
		t.Fatalf("parseFloat() = %v, want 42.5", f)
	}
}

func TestParseFloatInvalid(t *testing.T) {
	if _, err := parseFloat("not-a-number", "brightness"); err == nil {
		t.Fatal("parseFloat() error = nil, want error")
	}
}

func TestParseIntValid(t *testing.T) {
	i, err := parseInt("2700", "kelvin")
	if err != nil {
		t.Fatalf("parseInt() error = %v", err)
	}
	if i != 2700 {
		t.Fatalf("parseInt() = %v, want 2700", i)
	}
}

func TestParseIntInvalid(t *testing.T) {
	if _, err := parseInt("not-a-number", "kelvin"); err == nil {
		t.Fatal("parseInt() error = nil, want error")
	}
}
