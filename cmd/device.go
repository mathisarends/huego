package cmd

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

type sensorView struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Device   string `json:"device"`
	DeviceID string `json:"device_id"`
	Enabled  *bool  `json:"enabled,omitempty"`
	Value    any    `json:"value,omitempty"`
	Unit     string `json:"unit,omitempty"`
	Changed  string `json:"changed,omitempty"`
	Button   int    `json:"button,omitempty"`
}

func newSensorView(device hue.DeviceInfo, s hue.Sensor) sensorView {
	view := sensorView{ID: s.ID, Type: string(s.Type), Device: device.GetName(), DeviceID: device.ID, Enabled: s.Enabled}
	if reading, ok := s.Reading(); ok {
		view.Value, view.Unit, view.Changed = reading.Value, reading.Unit, reading.Changed
	}
	if s.Metadata != nil {
		view.Button = s.Metadata.ControlID
	}
	return view
}

func (v sensorView) valueText() string {
	switch value := v.Value.(type) {
	case nil:
		return "-"
	case bool:
		if value {
			return "motion"
		}
		return "no motion"
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64) + " " + v.Unit
	default:
		text := fmt.Sprint(value)
		if v.Button > 0 {
			text = fmt.Sprintf("button %d: %s", v.Button, text)
		}
		return text
	}
}

func (v sensorView) enabledText() string {
	if v.Enabled == nil {
		return "-"
	}
	return yesNo(*v.Enabled)
}

func (v sensorView) summary() string {
	text := strings.ReplaceAll(v.Type, "_", " ") + " " + v.valueText()
	if v.Enabled != nil && !*v.Enabled {
		text += " (disabled)"
	}
	return text
}

var sensorListing = listing[sensorView]{
	headers: []string{"DEVICE", "TYPE", "VALUE", "ENABLED", "CHANGED", "ID"},
	row: func(v sensorView) []string {
		return []string{v.Device, v.Type, v.valueText(), v.enabledText(), changedText(v.Changed), v.ID}
	},
}

func changedText(changed string) string {
	at, err := time.Parse(time.RFC3339, changed)
	if err != nil {
		return orDash(changed)
	}
	return at.Local().Format(time.DateTime)
}

type batteryView struct {
	State string `json:"state"`
	Level *int   `json:"level,omitempty"`
}

type deviceView struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Product         string       `json:"product"`
	Model           string       `json:"model"`
	Manufacturer    string       `json:"manufacturer"`
	Archetype       string       `json:"archetype"`
	SoftwareVersion string       `json:"software_version"`
	Connectivity    string       `json:"connectivity,omitempty"`
	Battery         *batteryView `json:"battery,omitempty"`
	Services        []serviceRef `json:"services"`
	Sensors         []sensorView `json:"sensors,omitempty"`
}

type serviceRef struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

func newDeviceView(d hue.DeviceInfo) deviceView {
	view := deviceView{
		ID:              d.ID,
		Name:            d.GetName(),
		Product:         d.ProductData.ProductName,
		Model:           d.ProductData.ModelID,
		Manufacturer:    d.ProductData.ManufacturerName,
		Archetype:       d.ProductData.ProductArchetype,
		SoftwareVersion: d.ProductData.SoftwareVersion,
		Connectivity:    d.Connectivity,
		Services:        make([]serviceRef, len(d.Services)),
	}
	if d.Battery != nil {
		view.Battery = &batteryView{State: d.Battery.State, Level: d.Battery.Level}
	}
	for i, s := range d.Services {
		view.Services[i] = serviceRef{ID: s.RID, Type: string(s.RType)}
	}
	for _, s := range d.Sensors {
		view.Sensors = append(view.Sensors, newSensorView(d, s))
	}
	return view
}

func (v deviceView) batteryText() string {
	switch {
	case v.Battery == nil:
		return "-"
	case v.Battery.Level == nil:
		return v.Battery.State
	case v.Battery.State == "" || v.Battery.State == "normal":
		return fmt.Sprintf("%d%%", *v.Battery.Level)
	default:
		return fmt.Sprintf("%d%% (%s)", *v.Battery.Level, v.Battery.State)
	}
}

var deviceListing = listing[deviceView]{
	headers: []string{"NAME", "PRODUCT", "CONNECTIVITY", "BATTERY", "SOFTWARE", "ID"},
	row: func(v deviceView) []string {
		return []string{v.Name, v.Product, orDash(v.Connectivity), v.batteryText(), v.SoftwareVersion, v.ID}
	},
	name: func(v deviceView) string { return v.Name },
}

func (v deviceView) fields() []field {
	services := make([]string, len(v.Services))
	for i, s := range v.Services {
		services[i] = s.Type
	}
	sensors := make([]string, len(v.Sensors))
	for i, s := range v.Sensors {
		sensors[i] = s.summary()
	}
	battery := ""
	if v.Battery != nil {
		battery = v.batteryText()
	}
	return []field{
		{"Name", v.Name},
		{"ID", v.ID},
		{"Product", v.Product},
		{"Model", v.Model},
		{"Manufacturer", v.Manufacturer},
		{"Software", v.SoftwareVersion},
		{"Connectivity", v.Connectivity},
		{"Battery", battery},
		{"Sensors", strings.Join(sensors, ", ")},
		{"Services", strings.Join(services, ", ")},
	}
}

var deviceCmd = &cobra.Command{
	Use:     "device",
	Aliases: []string{"devices"},
	Short:   "Inspect devices: product, firmware, connectivity, and battery",
}

var deviceListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List devices with their product, connectivity, battery, and firmware",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		devices, err := client.Devices.ListDetailed(cmd.Context())
		if err != nil {
			return err
		}
		return writeList(cmd, mapViews(devices, newDeviceView), deviceListing)
	},
}

var deviceGetCmd = &cobra.Command{
	Use:   "get <target>",
	Short: "Show one device with its services and sensor readings",
	Long: "Show one device with its services and sensor readings. target is the device's\n" +
		"name or ID, or the ID of one of its services, such as a light or sensor ID.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		device, err := client.Devices.FindDetailed(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		view := newDeviceView(device)
		return writeDetails(cmd, view, view.fields())
	},
}

var sensorCmd = &cobra.Command{
	Use:     "sensor",
	Aliases: []string{"sensors"},
	Short:   "Read motion, temperature, light level, contact, and button sensors, or switch them off",
}

var sensorListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List sensors with their latest reading",
	Long: "List sensors with their latest reading. Values are motion (true/false),\n" +
		"temperature (°C), light level (lux), contact (open/closed), and the last button event.",
	Example: "  huego sensor list\n  huego sensor list --type motion,temperature --json",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		types, _ := cmd.Flags().GetStringSlice("type")
		for _, t := range types {
			if !slices.Contains(hue.SensorTypes, hue.ResourceType(t)) {
				return usagef("unknown sensor type %q: use %s", t, sensorTypeNames())
			}
		}

		client, err := newClient()
		if err != nil {
			return err
		}
		devices, err := client.Devices.ListDetailed(cmd.Context())
		if err != nil {
			return err
		}
		var sensors []sensorView
		for _, device := range devices {
			for _, sensor := range device.Sensors {
				if len(types) == 0 || slices.Contains(types, string(sensor.Type)) {
					sensors = append(sensors, newSensorView(device, sensor))
				}
			}
		}
		return writeList(cmd, sensors, sensorListing)
	},
}

func sensorTypeNames() string {
	names := make([]string, len(hue.SensorTypes))
	for i, t := range hue.SensorTypes {
		names[i] = string(t)
	}
	return strings.Join(names, ", ")
}

func newSensorSwitchCommand(use, short string, enabled bool) *cobra.Command {
	return &cobra.Command{
		Use:   use + " <device>",
		Short: short,
		Long: short + ".\ndevice is the device's name or ID, or the ID of one of its sensors.\n" +
			"A disabled motion sensor no longer triggers its automations.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newClient()
			if err != nil {
				return err
			}
			device, err := client.Devices.FindDetailed(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			resp, err := client.Devices.SetSensorsEnabled(cmd.Context(), device, enabled)
			if err != nil {
				return err
			}
			return writeResponse(cmd, device.GetName(), resp)
		},
	}
}

type bridgeView struct {
	ID              string `json:"id"`
	BridgeID        string `json:"bridge_id"`
	Name            string `json:"name"`
	Address         string `json:"address"`
	Model           string `json:"model"`
	SoftwareVersion string `json:"software_version"`
	TimeZone        string `json:"time_zone"`
	DeviceID        string `json:"device_id"`
}

func (v bridgeView) fields() []field {
	return []field{
		{"Name", v.Name},
		{"Bridge ID", v.BridgeID},
		{"Address", v.Address},
		{"Model", v.Model},
		{"Software", v.SoftwareVersion},
		{"Time zone", v.TimeZone},
		{"ID", v.ID},
		{"Device ID", v.DeviceID},
	}
}

var bridgeCmd = &cobra.Command{
	Use:   "bridge",
	Short: "Inspect the bridge huego is connected to",
}

var bridgeGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show the bridge's ID, address, model, firmware, and time zone",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		bridge, device, err := client.Bridge(cmd.Context())
		if err != nil {
			return err
		}
		view := bridgeView{
			ID:              bridge.ID,
			BridgeID:        bridge.BridgeID,
			Name:            device.GetName(),
			Address:         client.Host(),
			Model:           device.ProductData.ModelID,
			SoftwareVersion: device.ProductData.SoftwareVersion,
			TimeZone:        bridge.TimeZone.TimeZone,
			DeviceID:        device.ID,
		}
		return writeDetails(cmd, view, view.fields())
	},
}

func init() {
	addNamesFlag(deviceListCmd)
	deviceCmd.AddCommand(deviceListCmd, deviceGetCmd)

	sensorListCmd.Flags().StringSlice("type", nil, "only these sensor types: "+sensorTypeNames())
	sensorCmd.AddCommand(
		sensorListCmd,
		newSensorSwitchCommand("enable", "Turn on all sensors of a device, like the Hue app's sensor switch", true),
		newSensorSwitchCommand("disable", "Turn off all sensors of a device, like the Hue app's sensor switch", false),
	)

	bridgeCmd.AddCommand(bridgeGetCmd)
	rootCmd.AddCommand(deviceCmd, sensorCmd, bridgeCmd)
}
