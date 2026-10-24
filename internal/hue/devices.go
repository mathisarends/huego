package hue

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"slices"
)

const (
	ResourceTypeMotion             ResourceType = "motion"
	ResourceTypeTemperature        ResourceType = "temperature"
	ResourceTypeLightLevel         ResourceType = "light_level"
	ResourceTypeContact            ResourceType = "contact"
	ResourceTypeButton             ResourceType = "button"
	ResourceTypeDevicePower        ResourceType = "device_power"
	ResourceTypeZigbeeConnectivity ResourceType = "zigbee_connectivity"
	ResourceTypeBridge             ResourceType = "bridge"
)

// SensorTypes are the resource types huego reads as sensors.
var SensorTypes = []ResourceType{
	ResourceTypeMotion, ResourceTypeTemperature, ResourceTypeLightLevel, ResourceTypeContact, ResourceTypeButton,
}

type ProductData struct {
	ModelID          string `json:"model_id"`
	ManufacturerName string `json:"manufacturer_name"`
	ProductName      string `json:"product_name"`
	ProductArchetype string `json:"product_archetype"`
	Certified        bool   `json:"certified"`
	SoftwareVersion  string `json:"software_version"`
}

type Device struct {
	ID          string              `json:"id"`
	Metadata    NamedMetadata       `json:"metadata"`
	ProductData ProductData         `json:"product_data"`
	Services    []ResourceReference `json:"services"`
}

func (d Device) GetID() string   { return d.ID }
func (d Device) GetName() string { return d.Metadata.Name }

// HasService reports whether id is one of the device's services, such as
// its light or motion sensor.
func (d Device) HasService(id string) bool {
	return slices.ContainsFunc(d.Services, func(s ResourceReference) bool { return s.RID == id })
}

type Battery struct {
	State string `json:"state"`
	Level *int   `json:"level,omitempty"`
}

type report struct {
	Changed string `json:"changed"`
}

// Sensor is any of the SensorTypes; only the field matching Type is set.
type Sensor struct {
	ID      string            `json:"id"`
	Type    ResourceType      `json:"type"`
	Owner   ResourceReference `json:"owner"`
	Enabled *bool             `json:"enabled,omitempty"`
	Motion  *struct {
		MotionValid  bool `json:"motion_valid"`
		MotionReport *struct {
			report
			Motion bool `json:"motion"`
		} `json:"motion_report"`
	} `json:"motion,omitempty"`
	Temperature *struct {
		TemperatureValid  bool `json:"temperature_valid"`
		TemperatureReport *struct {
			report
			Temperature float64 `json:"temperature"`
		} `json:"temperature_report"`
	} `json:"temperature,omitempty"`
	Light *struct {
		LightLevelValid  bool `json:"light_level_valid"`
		LightLevelReport *struct {
			report
			LightLevel int `json:"light_level"`
		} `json:"light_level_report"`
	} `json:"light,omitempty"`
	ContactReport *struct {
		report
		State string `json:"state"`
	} `json:"contact_report,omitempty"`
	Button *struct {
		ButtonReport *struct {
			Updated string `json:"updated"`
			Event   string `json:"event"`
		} `json:"button_report"`
	} `json:"button,omitempty"`
	Metadata *struct {
		ControlID int `json:"control_id"`
	} `json:"metadata,omitempty"`
}

// Reading is a sensor's latest report: Value is a bool for motion, a number
// for temperature (°C) and light level (lux), and a string for contact
// ("open"/"closed") and button events. ok is false before the first report.
type Reading struct {
	Value   any
	Unit    string
	Changed string
}

func (s Sensor) Reading() (Reading, bool) {
	switch {
	case s.Motion != nil && s.Motion.MotionValid && s.Motion.MotionReport != nil:
		r := s.Motion.MotionReport
		return Reading{Value: r.Motion, Changed: r.Changed}, true
	case s.Temperature != nil && s.Temperature.TemperatureValid && s.Temperature.TemperatureReport != nil:
		r := s.Temperature.TemperatureReport
		return Reading{Value: math.Round(r.Temperature*10) / 10, Unit: "°C", Changed: r.Changed}, true
	case s.Light != nil && s.Light.LightLevelValid && s.Light.LightLevelReport != nil:
		r := s.Light.LightLevelReport
		return Reading{Value: Lux(r.LightLevel), Unit: "lux", Changed: r.Changed}, true
	case s.ContactReport != nil:
		state := "open"
		if s.ContactReport.State == "contact" {
			state = "closed"
		}
		return Reading{Value: state, Changed: s.ContactReport.Changed}, true
	case s.Button != nil && s.Button.ButtonReport != nil:
		r := s.Button.ButtonReport
		return Reading{Value: r.Event, Changed: r.Updated}, true
	}
	return Reading{}, false
}

// Lux converts the bridge's logarithmic light level (10000·log10(lux)+1).
func Lux(lightLevel int) float64 {
	return math.Round(math.Pow(10, float64(lightLevel-1)/10000))
}

// DeviceInfo is a device together with the state of its battery,
// connectivity, and sensors.
type DeviceInfo struct {
	Device
	Connectivity string
	Battery      *Battery
	Sensors      []Sensor
}

type Devices struct {
	resourceNamespace[Device]
}

func newDevices(client *Client) *Devices {
	return &Devices{resourceNamespace[Device]{client: client, resourceType: string(ResourceTypeDevice)}}
}

// ListDetailed returns every device with its battery, connectivity, and
// sensors, read from the bridge in a single request.
func (d *Devices) ListDetailed(ctx context.Context) ([]DeviceInfo, error) {
	var resp APIResponse[json.RawMessage]
	if err := d.client.get(ctx, "", &resp); err != nil {
		return nil, err
	}

	var devices []DeviceInfo
	connectivity := map[string]string{}
	batteries := map[string]*Battery{}
	sensors := map[string][]Sensor{}
	for _, raw := range resp.Data {
		var head struct {
			Type  ResourceType      `json:"type"`
			Owner ResourceReference `json:"owner"`
		}
		if err := json.Unmarshal(raw, &head); err != nil {
			return nil, err
		}
		switch {
		case head.Type == ResourceTypeDevice:
			var device Device
			if err := json.Unmarshal(raw, &device); err != nil {
				return nil, err
			}
			devices = append(devices, DeviceInfo{Device: device})
		case head.Type == ResourceTypeZigbeeConnectivity:
			var zigbee struct {
				Status string `json:"status"`
			}
			if err := json.Unmarshal(raw, &zigbee); err != nil {
				return nil, err
			}
			connectivity[head.Owner.RID] = zigbee.Status
		case head.Type == ResourceTypeDevicePower:
			var power struct {
				PowerState struct {
					BatteryState string `json:"battery_state"`
					BatteryLevel *int   `json:"battery_level"`
				} `json:"power_state"`
			}
			if err := json.Unmarshal(raw, &power); err != nil {
				return nil, err
			}
			if power.PowerState.BatteryState != "" || power.PowerState.BatteryLevel != nil {
				batteries[head.Owner.RID] = &Battery{State: power.PowerState.BatteryState, Level: power.PowerState.BatteryLevel}
			}
		case slices.Contains(SensorTypes, head.Type):
			var sensor Sensor
			if err := json.Unmarshal(raw, &sensor); err != nil {
				return nil, err
			}
			sensors[head.Owner.RID] = append(sensors[head.Owner.RID], sensor)
		}
	}

	for i := range devices {
		id := devices[i].ID
		devices[i].Connectivity = connectivity[id]
		devices[i].Battery = batteries[id]
		devices[i].Sensors = sensors[id]
	}
	return devices, nil
}

// FindDetailed resolves target to one device: its ID, the ID of any of its
// services (so a light's ID finds the light's device), or its name.
func (d *Devices) FindDetailed(ctx context.Context, target string) (DeviceInfo, error) {
	devices, err := d.ListDetailed(ctx)
	if err != nil {
		return DeviceInfo{}, err
	}
	if isUUID(target) {
		for _, device := range devices {
			if device.ID == target || device.HasService(target) {
				return device, nil
			}
		}
		return DeviceInfo{}, newResourceNotFoundError("no device or device service with ID %s", target)
	}
	return matchName(string(ResourceTypeDevice), devices, target, DeviceInfo.GetName)
}

// SetSensorsEnabled turns all of a device's sensors on or off, as the Hue
// app's sensor switch does.
func (d *Devices) SetSensorsEnabled(ctx context.Context, device DeviceInfo, enabled bool) (APIResponse[ResourceIdentifier], error) {
	var combined APIResponse[ResourceIdentifier]
	for _, sensor := range device.Sensors {
		if sensor.Enabled == nil {
			continue
		}
		var resp APIResponse[ResourceIdentifier]
		err := d.client.request(ctx, http.MethodPut, string(sensor.Type)+"/"+sensor.ID, map[string]bool{"enabled": enabled}, &resp)
		if err != nil {
			return combined, err
		}
		combined.Errors = append(combined.Errors, resp.Errors...)
		combined.Data = append(combined.Data, resp.Data...)
	}
	if combined.Data == nil && combined.Errors == nil {
		return combined, newResourceNotFoundError("device %q has no sensors that can be switched", device.GetName())
	}
	return combined, nil
}

type BridgeInfo struct {
	ID       string `json:"id"`
	BridgeID string `json:"bridge_id"`
	TimeZone struct {
		TimeZone string `json:"time_zone"`
	} `json:"time_zone"`
	Owner ResourceReference `json:"owner"`
}

// Bridge returns the bridge resource and the device that hosts it.
func (c *Client) Bridge(ctx context.Context) (BridgeInfo, Device, error) {
	var resp APIResponse[BridgeInfo]
	if err := c.get(ctx, string(ResourceTypeBridge), &resp); err != nil {
		return BridgeInfo{}, Device{}, err
	}
	if len(resp.Data) == 0 {
		return BridgeInfo{}, Device{}, newResourceNotFoundError("bridge reported no bridge resource")
	}
	bridge := resp.Data[0]
	device, err := c.Devices.GetOne(ctx, bridge.Owner.RID)
	return bridge, device, err
}

// Host is the bridge address the client talks to.
func (c *Client) Host() string { return c.host }
