package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

func parseFloat(value, name string) (float64, error) {
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: expected a number", name, value)
	}
	return f, nil
}

func parseInt(value, name string) (int, error) {
	i, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: expected a whole number", name, value)
	}
	return i, nil
}

func parsePercentage(value, name string) (float64, error) {
	percentage, err := parseFloat(value, name)
	if err != nil {
		return 0, err
	}
	if percentage < 0 || percentage > 100 {
		return 0, fmt.Errorf("invalid %s %q: must be between 0 and 100", name, value)
	}
	return percentage, nil
}

func parseKelvin(value string) (int, error) {
	kelvin, err := parseInt(value, "kelvin")
	if err != nil {
		return 0, err
	}
	if kelvin < hue.KelvinMinimum || kelvin > hue.KelvinMaximum {
		return 0, fmt.Errorf("invalid kelvin %q: must be between %d and %d", value, hue.KelvinMinimum, hue.KelvinMaximum)
	}
	return kelvin, nil
}

func readableNumericFlagError(name string) func(*cobra.Command, error) error {
	return func(_ *cobra.Command, err error) error {
		message := err.Error()
		marker := " in -"
		index := strings.LastIndex(message, marker)
		if index == -1 {
			return err
		}
		value := message[index+len(marker)-1:]
		if _, parseErr := strconv.ParseFloat(value, 64); parseErr != nil {
			return err
		}
		return fmt.Errorf("invalid %s %q: negative values are not allowed", name, value)
	}
}
