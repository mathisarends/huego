package hue

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type RGB struct {
	R, G, B uint8
}

type ColorXY struct {
	X, Y float64
}

const (
	MirekMinimum = 153
	MirekMaximum = 500
)

var namedColors = map[string]RGB{
	"black":      {0, 0, 0},
	"white":      {255, 255, 255},
	"red":        {255, 0, 0},
	"green":      {0, 255, 0},
	"blue":       {0, 0, 255},
	"yellow":     {255, 255, 0},
	"cyan":       {0, 255, 255},
	"magenta":    {255, 0, 255},
	"orange":     {255, 165, 0},
	"pink":       {255, 105, 180},
	"purple":     {128, 0, 128},
	"violet":     {138, 43, 226},
	"turquoise":  {64, 224, 208},
	"lime":       {50, 205, 50},
	"gold":       {255, 215, 0},
	"warm_white": {255, 180, 107},
	"cool_white": {212, 235, 255},
}

// ParseColor resolves a hex string ("#ff8800", "#f80") or a named color
// (see namedColors) to RGB.
func ParseColor(color string) (RGB, error) {
	normalized := strings.ToLower(strings.TrimSpace(color))
	normalized = strings.NewReplacer(" ", "_", "-", "_").Replace(normalized)
	if rgb, ok := namedColors[normalized]; ok {
		return rgb, nil
	}
	rgb, err := HexToRGB(color)
	if err != nil {
		if strings.HasPrefix(strings.TrimSpace(color), "#") {
			return RGB{}, err
		}
		return RGB{}, fmt.Errorf("unknown color %q: expected a hex string like '#ff8800' or a named color", color)
	}
	return rgb, nil
}

func HexToRGB(hexColor string) (RGB, error) {
	value := strings.TrimPrefix(strings.TrimSpace(hexColor), "#")
	if len(value) == 3 {
		expanded := make([]byte, 0, 6)
		for i := range 3 {
			expanded = append(expanded, value[i], value[i])
		}
		value = string(expanded)
	}
	if len(value) != 6 {
		return RGB{}, fmt.Errorf("invalid hex color %q: expected '#rgb' or '#rrggbb'", hexColor)
	}
	r, err1 := strconv.ParseUint(value[0:2], 16, 8)
	g, err2 := strconv.ParseUint(value[2:4], 16, 8)
	b, err3 := strconv.ParseUint(value[4:6], 16, 8)
	if err1 != nil || err2 != nil || err3 != nil {
		return RGB{}, fmt.Errorf("invalid hex color %q", hexColor)
	}
	return RGB{R: uint8(r), G: uint8(g), B: uint8(b)}, nil
}

func RGBToXY(rgb RGB) ColorXY {
	red := gammaExpand(float64(rgb.R) / 255)
	green := gammaExpand(float64(rgb.G) / 255)
	blue := gammaExpand(float64(rgb.B) / 255)

	x := red*0.649926 + green*0.103455 + blue*0.197109
	y := red*0.234327 + green*0.743075 + blue*0.022598
	z := green*0.053077 + blue*1.035763

	total := x + y + z
	if total == 0 {
		return ColorXY{X: 0, Y: 0}
	}
	return ColorXY{
		X: math.Round(x/total*10000) / 10000,
		Y: math.Round(y/total*10000) / 10000,
	}
}

func XYToRGB(xy ColorXY, brightness float64) RGB {
	if xy.Y == 0 {
		return RGB{0, 0, 0}
	}

	luminance := math.Max(0, math.Min(brightness, 100)) / 100
	x := (luminance / xy.Y) * xy.X
	z := (luminance / xy.Y) * (1 - xy.X - xy.Y)

	red := x*1.656492 - luminance*0.354851 - z*0.255038
	green := -x*0.707196 + luminance*1.655397 + z*0.036152
	blue := x*0.051713 - luminance*0.121364 + z*1.011530

	channels := [3]float64{gammaCompress(red), gammaCompress(green), gammaCompress(blue)}
	peak := channels[0]
	for _, c := range channels[1:] {
		if c > peak {
			peak = c
		}
	}
	if peak > 1 {
		for i := range channels {
			channels[i] /= peak
		}
	}

	clamp := func(c float64) uint8 {
		return uint8(math.Round(math.Max(0, math.Min(c, 1)) * 255))
	}
	return RGB{R: clamp(channels[0]), G: clamp(channels[1]), B: clamp(channels[2])}
}

func XYToHex(xy ColorXY, brightness float64) string {
	rgb := XYToRGB(xy, brightness)
	return fmt.Sprintf("#%02x%02x%02x", rgb.R, rgb.G, rgb.B)
}

func KelvinToMirek(kelvin int) (int, error) {
	if kelvin <= 0 {
		return 0, fmt.Errorf("color temperature in kelvin must be positive")
	}
	return clampMirek(int(math.Round(1_000_000 / float64(kelvin)))), nil
}

func MirekToKelvin(mirek int) (int, error) {
	if mirek <= 0 {
		return 0, fmt.Errorf("color temperature in mirek must be positive")
	}
	return int(math.Round(1_000_000 / float64(mirek))), nil
}

func clampMirek(mirek int) int {
	if mirek < MirekMinimum {
		return MirekMinimum
	}
	if mirek > MirekMaximum {
		return MirekMaximum
	}
	return mirek
}

func gammaExpand(channel float64) float64 {
	if channel > 0.04045 {
		return math.Pow((channel+0.055)/1.055, 2.4)
	}
	return channel / 12.92
}

func gammaCompress(channel float64) float64 {
	if channel <= 0.0031308 {
		return 12.92 * channel
	}
	return 1.055*math.Pow(math.Max(channel, 0), 1/2.4) - 0.055
}
