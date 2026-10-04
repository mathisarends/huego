package hue

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"
)

//go:embed api_catalog.json
var catalogJSON []byte

type ResourceCapability struct {
	ResourceType      string                     `json:"resource_type"`
	CollectionMethods []string                   `json:"collection_methods"`
	ItemMethods       []string                   `json:"item_methods"`
	RequestSchemas    map[string]json.RawMessage `json:"request_schemas,omitempty"`
}

type APICatalog struct {
	Source    string               `json:"source"`
	Revision  string               `json:"revision"`
	Resources []ResourceCapability `json:"resources"`
}

func Catalog() APICatalog {
	var catalog APICatalog
	if err := json.Unmarshal(catalogJSON, &catalog); err != nil {
		panic(err)
	}
	return catalog
}

func Capability(kind string) (ResourceCapability, error) {
	for _, capability := range Catalog().Resources {
		if capability.ResourceType == kind {
			return capability, nil
		}
	}
	return ResourceCapability{}, fmt.Errorf("unknown resource type %q; use resource types or --raw for firmware extensions", kind)
}

func (c ResourceCapability) Supports(method string, item bool) bool {
	methods := c.CollectionMethods
	if item {
		methods = c.ItemMethods
	}
	return slices.Contains(methods, method)
}
