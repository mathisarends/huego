package hue

import (
	"strings"
	"testing"
)

func TestParseColorNamed(t *testing.T) {
	rgb, err := ParseColor("warm white")
	if err != nil {
		t.Fatalf("ParseColor() error = %v", err)
	}
	if want := (RGB{255, 180, 107}); rgb != want {
		t.Fatalf("ParseColor() = %+v, want %+v", rgb, want)
	}
}

func TestParseColorHex(t *testing.T) {
	rgb, err := ParseColor("#ff8800")
	if err != nil {
		t.Fatalf("ParseColor() error = %v", err)
	}
	if want := (RGB{0xff, 0x88, 0x00}); rgb != want {
		t.Fatalf("ParseColor() = %+v, want %+v", rgb, want)
	}
}

func TestParseColorUnknown(t *testing.T) {
	if _, err := ParseColor("not-a-color"); err == nil {
		t.Fatal("ParseColor() error = nil, want error")
	}
}

func TestHexToRGBShorthand(t *testing.T) {
	rgb, err := HexToRGB("#f80")
	if err != nil {
		t.Fatalf("HexToRGB() error = %v", err)
	}
	if want := (RGB{0xff, 0x88, 0x00}); rgb != want {
		t.Fatalf("HexToRGB() = %+v, want %+v", rgb, want)
	}
}

func TestRGBXYRoundTrip(t *testing.T) {
	original := RGB{255, 0, 0}
	xy := RGBToXY(original)
	back := XYToRGB(xy, 100)
	if back.R < 240 {
		t.Fatalf("XYToRGB(RGBToXY(%v)) = %+v, want red channel to dominate", original, back)
	}
}

func TestXYToHex(t *testing.T) {
	xy := RGBToXY(RGB{255, 0, 0})
	hex := XYToHex(xy, 100)
	if !strings.HasPrefix(hex, "#ff00") {
		t.Fatalf("XYToHex() = %q, want a hex string with red channel dominant", hex)
	}
}

func TestKelvinMirekRoundTrip(t *testing.T) {
	mirek, err := KelvinToMirek(2700)
	if err != nil {
		t.Fatalf("KelvinToMirek() error = %v", err)
	}
	if mirek != 370 {
		t.Fatalf("KelvinToMirek(2700) = %d, want 370", mirek)
	}

	kelvin, err := MirekToKelvin(mirek)
	if err != nil {
		t.Fatalf("MirekToKelvin() error = %v", err)
	}
	if kelvin != 2703 {
		t.Fatalf("MirekToKelvin(370) = %d, want 2703", kelvin)
	}
}

func TestKelvinToMirekRejectsOutOfRange(t *testing.T) {
	if _, err := KelvinToMirek(10000); err == nil {
		t.Fatal("KelvinToMirek(10000) error = nil, want range error")
	}
}

func TestKelvinToMirekRejectsNonPositive(t *testing.T) {
	if _, err := KelvinToMirek(0); err == nil {
		t.Fatal("KelvinToMirek(0) error = nil, want error")
	}
}

func TestMirekToKelvinRejectsNonPositive(t *testing.T) {
	if _, err := MirekToKelvin(0); err == nil {
		t.Fatal("MirekToKelvin(0) error = nil, want error")
	}
}
