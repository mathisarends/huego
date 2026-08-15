package cmd

import (
	"fmt"
	"strconv"
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
