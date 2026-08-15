package cmd

import (
	"github.com/mathisarends/huego/internal/config"
	"github.com/mathisarends/huego/internal/hue"
)

// newClient is a var so tests can substitute a client pointed at a fake bridge.
var newClient = func() (*hue.Client, error) {
	bridge, err := config.LoadBridge()
	if err != nil {
		return nil, err
	}
	return hue.NewClient(bridge), nil
}
