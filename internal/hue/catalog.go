package hue

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"

	_ "embed"
)

//go:embed api_catalog.json
var catalogJSON []byte

//go:embed api_extensions.json
var extensionJSON []byte

type ResourceCapability struct {
	ResourceType      string                     `json:"resource_type"`
	CollectionMethods []string                   `json:"collection_methods"`
	ItemMethods       []string                   `json:"item_methods"`
	RequestSchemas    map[string]json.RawMessage `json:"request_schemas,omitempty"`
	RequestExamples   map[string]json.RawMessage `json:"request_examples,omitempty"`
	Notes             []string                   `json:"notes,omitempty"`
}

type APICatalog struct {
	Source            string               `json:"source"`
	Revision          string               `json:"revision"`
	Resources         []ResourceCapability `json:"resources"`
	AdditionalSources []string             `json:"additional_sources,omitempty"`
}

func PinnedCatalog() APICatalog {
	var catalog APICatalog
	if err := json.Unmarshal(catalogJSON, &catalog); err != nil {
		panic(err)
	}
	return catalog
}

func Catalog() APICatalog {
	catalog := PinnedCatalog()
	var extensions struct {
		Source              string               `json:"source"`
		VerificationSources []string             `json:"verification_sources"`
		Resources           []ResourceCapability `json:"resources"`
	}
	if err := json.Unmarshal(extensionJSON, &extensions); err != nil {
		panic(err)
	}
	catalog.AdditionalSources = append([]string{extensions.Source}, extensions.VerificationSources...)
	for _, extension := range extensions.Resources {
		index := slices.IndexFunc(catalog.Resources, func(resource ResourceCapability) bool { return resource.ResourceType == extension.ResourceType })
		if index < 0 {
			catalog.Resources = append(catalog.Resources, extension)
			continue
		}
		resource := &catalog.Resources[index]
		resource.CollectionMethods, resource.ItemMethods = extension.CollectionMethods, extension.ItemMethods
		resource.RequestExamples, resource.Notes = extension.RequestExamples, extension.Notes
		for method, schema := range extension.RequestSchemas {
			resource.RequestSchemas[method] = mergeSchema(resource.RequestSchemas[method], schema)
		}
	}
	sort.Slice(catalog.Resources, func(i, j int) bool { return catalog.Resources[i].ResourceType < catalog.Resources[j].ResourceType })
	return catalog
}

func mergeSchema(base, patch json.RawMessage) json.RawMessage {
	var left, right map[string]json.RawMessage
	_ = json.Unmarshal(base, &left)
	_ = json.Unmarshal(patch, &right)
	if left == nil {
		return patch
	}
	for key, value := range right {
		if len(value) > 0 && value[0] == '{' {
			left[key] = mergeSchema(left[key], value)
		} else {
			left[key] = value
		}
	}
	result, _ := json.Marshal(left)
	return result
}

func Capability(kind string) (ResourceCapability, error) {
	for _, capability := range Catalog().Resources {
		if capability.ResourceType == kind {
			return capability, nil
		}
	}
	return ResourceCapability{}, &Problem{Code: "unknown_resource_type", Message: fmt.Sprintf("unknown resource type %q; use resource types or --raw for firmware extensions", kind)}
}

func (c ResourceCapability) Supports(method string, item bool) bool {
	methods := c.CollectionMethods
	if item {
		methods = c.ItemMethods
	}
	return slices.Contains(methods, method)
}
