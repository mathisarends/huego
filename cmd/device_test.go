package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

const (
	sensorDeviceID = "11111111-1111-1111-1111-111111111111"
	motionID       = "22222222-2222-2222-2222-222222222222"
	temperatureID  = "33333333-3333-3333-3333-333333333333"
	bridgeDeviceID = "44444444-4444-4444-4444-444444444444"
)

const allResourcesFixture = `{"errors":[],"data":[
{"id":"` + sensorDeviceID + `","type":"device","metadata":{"name":"Hallway sensor","archetype":"unknown_archetype"},
 "product_data":{"model_id":"SML003","manufacturer_name":"Signify Netherlands B.V.","product_name":"Hue motion sensor","product_archetype":"unknown_archetype","certified":true,"software_version":"2.53.6"},
 "services":[{"rid":"` + motionID + `","rtype":"motion"},{"rid":"` + temperatureID + `","rtype":"temperature"},{"rid":"p1","rtype":"device_power"}]},
{"id":"` + bridgeDeviceID + `","type":"device","metadata":{"name":"Hue Bridge","archetype":"bridge_v2"},
 "product_data":{"model_id":"BSB002","product_name":"Hue Bridge","software_version":"1.66.1966060010"},"services":[{"rid":"b1","rtype":"bridge"}]},
{"id":"` + motionID + `","type":"motion","owner":{"rid":"` + sensorDeviceID + `","rtype":"device"},"enabled":true,
 "motion":{"motion":true,"motion_valid":true,"motion_report":{"changed":"2026-10-24T08:00:00Z","motion":true}}},
{"id":"` + temperatureID + `","type":"temperature","owner":{"rid":"` + sensorDeviceID + `","rtype":"device"},"enabled":true,
 "temperature":{"temperature":21.37,"temperature_valid":true,"temperature_report":{"changed":"2026-10-24T08:00:00Z","temperature":21.37}}},
{"id":"p1","type":"device_power","owner":{"rid":"` + sensorDeviceID + `","rtype":"device"},"power_state":{"battery_state":"low","battery_level":12}},
{"id":"z1","type":"zigbee_connectivity","owner":{"rid":"` + sensorDeviceID + `","rtype":"device"},"status":"connected"},
{"id":"l1","type":"light","owner":{"rid":"` + sensorDeviceID + `","rtype":"device"}}
]}`

func newDeviceBridge(t *testing.T, puts *[]string) {
	t.Helper()
	withClient(t, newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/clip/v2/resource")
		switch {
		case r.Method == http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			*puts = append(*puts, path+" "+string(body))
			id := path[strings.LastIndex(path, "/")+1:]
			_, _ = w.Write([]byte(`{"errors":[],"data":[{"rid":"` + id + `","rtype":"motion"}]}`))
		case path == "":
			_, _ = w.Write([]byte(allResourcesFixture))
		case path == "/bridge":
			_, _ = w.Write([]byte(`{"errors":[],"data":[{"id":"b1","bridge_id":"ecb5fafffe000000","time_zone":{"time_zone":"Europe/Berlin"},"owner":{"rid":"` + bridgeDeviceID + `","rtype":"device"}}]}`))
		case path == "/device/"+bridgeDeviceID:
			_, _ = w.Write([]byte(`{"errors":[],"data":[{"id":"` + bridgeDeviceID + `","metadata":{"name":"Hue Bridge"},"product_data":{"model_id":"BSB002","software_version":"1.66.1966060010"}}]}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestDeviceListShowsBatteryAndConnectivity(t *testing.T) {
	newDeviceBridge(t, nil)

	out, err := execute(t, "device", "list")
	if err != nil {
		t.Fatalf("device list: %v", err)
	}
	for _, want := range []string{"Hallway sensor", "Hue motion sensor", "connected", "12% (low)", "2.53.6", "Hue Bridge"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestDeviceGetResolvesServiceIDs(t *testing.T) {
	newDeviceBridge(t, nil)

	out, err := execute(t, "device", "get", motionID, "--json")
	if err != nil {
		t.Fatalf("device get: %v", err)
	}
	var view deviceView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("output %q is not JSON: %v", out, err)
	}
	if view.ID != sensorDeviceID || len(view.Sensors) != 2 || view.Battery == nil || *view.Battery.Level != 12 {
		t.Fatalf("view = %+v", view)
	}
}

func TestDeviceGetUnknownNameIsNotFound(t *testing.T) {
	newDeviceBridge(t, nil)

	_, err := execute(t, "device", "get", "Attic")
	if report, code := classifyError(err); code != exitNotFound {
		t.Fatalf("err = %v (%s), want not found", err, report.Code)
	}
}

func TestSensorListReadings(t *testing.T) {
	newDeviceBridge(t, nil)

	out, err := execute(t, "sensor", "list", "--json")
	if err != nil {
		t.Fatalf("sensor list: %v", err)
	}
	var sensors []sensorView
	if err := json.Unmarshal([]byte(out), &sensors); err != nil {
		t.Fatalf("output %q is not JSON: %v", out, err)
	}
	if len(sensors) != 2 || sensors[0].Value != true || sensors[1].Value != 21.4 || sensors[1].Unit != "°C" || sensors[0].Device != "Hallway sensor" {
		t.Fatalf("sensors = %+v", sensors)
	}

	out, err = execute(t, "sensor", "list", "--type", "temperature")
	if err != nil {
		t.Fatalf("sensor list --type: %v", err)
	}
	if !strings.Contains(out, "21.4 °C") || strings.Contains(out, "motion") {
		t.Fatalf("filtered output:\n%s", out)
	}
}

func TestSensorListRejectsUnknownType(t *testing.T) {
	_, err := execute(t, "sensor", "list", "--type", "humidity")
	if _, code := classifyError(err); code != exitUsage {
		t.Fatalf("err = %v, want usage error", err)
	}
}

func TestSensorDisableSwitchesEverySensorOfTheDevice(t *testing.T) {
	var puts []string
	newDeviceBridge(t, &puts)

	if _, err := execute(t, "sensor", "disable", "hallway sensor"); err != nil {
		t.Fatalf("sensor disable: %v", err)
	}
	want := []string{
		"/motion/" + motionID + ` {"enabled":false}`,
		"/temperature/" + temperatureID + ` {"enabled":false}`,
	}
	if strings.Join(puts, "\n") != strings.Join(want, "\n") {
		t.Fatalf("puts = %q, want %q", puts, want)
	}
}

func TestBridgeGet(t *testing.T) {
	newDeviceBridge(t, nil)

	out, err := execute(t, "bridge", "get")
	if err != nil {
		t.Fatalf("bridge get: %v", err)
	}
	for _, want := range []string{"ecb5fafffe000000", "BSB002", "1.66.1966060010", "Europe/Berlin", "127.0.0.1"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestSummarizeSensorEvents(t *testing.T) {
	tests := map[string]string{
		`{"motion":{"motion":true,"motion_valid":true,"motion_report":{"changed":"x","motion":true}}}`:        "motion",
		`{"motion":{"motion":false,"motion_valid":true,"motion_report":{"changed":"x","motion":false}}}`:      "no motion",
		`{"temperature":{"temperature_valid":true,"temperature_report":{"changed":"x","temperature":19.04}}}`: "temperature=19°C",
		`{"light":{"light_level_valid":true,"light_level_report":{"changed":"x","light_level":20001}}}`:       "light_level=100lux",
		`{"contact_report":{"changed":"x","state":"no_contact"}}`:                                             "contact=open",
		`{"power_state":{"battery_state":"normal","battery_level":80}}`:                                       "battery=80%",
	}
	for data, want := range tests {
		if got := summarizeChange(json.RawMessage(data)); got != want {
			t.Errorf("summarizeChange(%s) = %q, want %q", data, got, want)
		}
	}
}

func TestSummarizeLightEvents(t *testing.T) {
	tests := map[string]string{
		`{"id":"x","type":"light","owner":{"rid":"d"},"on":{"on":true}}`: "on",
		`{"dimming":{"brightness":42.6}}`:                                "brightness=43%",
		`{"color_temperature":{"mirek":370,"mirek_valid":true}}`:         "temperature=2703K",
		`{"color_temperature":{"mirek":null,"mirek_valid":false}}`:       `color_temperature={"mirek":null,"mirek_valid":false}`,
		`{"color":{"xy":{"x":0.6915,"y":0.3083}}}`:                       "color=#ff2c00",
		`{"status":"active","on":{"on":false}}`:                          "off status=active",
		`[1,2]`:                                                          "[1,2]",
	}
	for data, want := range tests {
		if got := summarizeChange(json.RawMessage(data)); got != want {
			t.Errorf("summarizeChange(%s) = %q, want %q", data, got, want)
		}
	}
}
