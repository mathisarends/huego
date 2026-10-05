package hue

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEveryRequestSchemaCompiles(t *testing.T) {
	for _, resource := range Catalog().Resources {
		for method := range resource.RequestSchemas {
			t.Run(resource.ResourceType+"/"+method, func(t *testing.T) {
				if _, err := CompileRequestSchema(resource.ResourceType, method); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestSchemaPreservesHueOnAndDescriptions(t *testing.T) {
	for _, kind := range []string{"light", "grouped_light"} {
		capability, _ := Capability(kind)
		var schema struct {
			Properties map[string]json.RawMessage `json:"properties"`
		}
		_ = json.Unmarshal(capability.RequestSchemas["PUT"], &schema)
		if len(schema.Properties["on"]) == 0 || len(schema.Properties["true"]) != 0 {
			t.Fatalf("corrupted on property for %s", kind)
		}
		if !strings.Contains(string(schema.Properties["on"]), "description") {
			t.Fatal("on description was discarded")
		}
		if err := ValidateRequest(kind, "PUT", json.RawMessage(`{"on":{"on":true}}`)); err != nil {
			t.Fatal(err)
		}
		if err := ValidateRequest(kind, "PUT", json.RawMessage(`{"on":{"on":"true"}}`)); err == nil {
			t.Fatal("accepted string as boolean")
		}
	}
	if err := ValidateRequest("scene", "POST", json.RawMessage(`{}`)); err == nil {
		t.Fatal("accepted scene without required fields")
	}
	if err := ValidateRequest("light", "PUT", json.RawMessage(`{"dimming":{"brightness":101}}`)); err == nil {
		t.Fatal("accepted out of range brightness")
	}
	if err := ValidateRequest("light", "PUT", json.RawMessage(`{"on":{"on":false},"future":null,"large":9007199254740993}`)); err != nil {
		t.Fatal(err)
	}
}

func TestSupplementalOperationsAndExamples(t *testing.T) {
	for _, kind := range []string{"behavior_instance", "entertainment_configuration"} {
		capability, err := Capability(kind)
		if err != nil {
			t.Fatal(err)
		}
		if !capability.Supports("POST", false) || !capability.Supports("DELETE", true) {
			t.Fatalf("missing lifecycle operations: %s", kind)
		}
	}
	for _, resource := range Catalog().Resources {
		for method, example := range resource.RequestExamples {
			if err := ValidateRequest(resource.ResourceType, method, example); err != nil {
				t.Fatalf("%s %s: %v", resource.ResourceType, method, err)
			}
		}
	}
}
