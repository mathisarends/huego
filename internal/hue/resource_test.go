package hue

import "testing"

func TestIsUUID(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"valid lowercase", "01234567-89ab-cdef-0123-456789abcdef", true},
		{"valid uppercase", "01234567-89AB-CDEF-0123-456789ABCDEF", true},
		{"too short", "01234567-89ab-cdef-0123", false},
		{"missing dashes", "0123456789abcdef0123456789abcdef01", false},
		{"non-hex character", "g1234567-89ab-cdef-0123-456789abcdef", false},
		{"plain name", "Desk Lamp", false},
		{"empty string", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUUID(tt.in); got != tt.want {
				t.Errorf("isUUID(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
