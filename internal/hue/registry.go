package hue

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

type Resource map[string]json.RawMessage

func (r Resource) GetID() string {
	var id string
	_ = json.Unmarshal(r["id"], &id)
	return id
}

func (r Resource) GetName() string {
	var metadata NamedMetadata
	_ = json.Unmarshal(r["metadata"], &metadata)
	if metadata.Name != "" {
		return metadata.Name
	}
	var name string
	_ = json.Unmarshal(r["name"], &name)
	return name
}

type ResourceResponse struct {
	Errors []json.RawMessage `json:"errors"`
	Data   []Resource        `json:"data"`
}

func (r ResourceResponse) Err() error {
	if len(r.Errors) == 0 {
		return nil
	}
	descriptions := make([]string, len(r.Errors))
	for i, raw := range r.Errors {
		var detail struct {
			Description string `json:"description"`
		}
		_ = json.Unmarshal(raw, &detail)
		descriptions[i] = detail.Description
		if descriptions[i] == "" {
			descriptions[i] = string(raw)
		}
	}
	return fmt.Errorf("hue bridge reported errors: %s", strings.Join(descriptions, "; "))
}

type ResourceRegistry struct {
	client *Client
}

func (r *ResourceRegistry) List(ctx context.Context) (ResourceResponse, error) {
	var response ResourceResponse
	err := r.client.get(ctx, "", &response)
	return response, err
}

type NativeNamespace struct {
	client       *Client
	resourceType string
	raw          bool
}

var resourceTypePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
var resourceIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

func (r *ResourceRegistry) Of(kind string, raw bool) (*NativeNamespace, error) {
	if !resourceTypePattern.MatchString(kind) {
		return nil, fmt.Errorf("invalid resource type %q: expected a CLIP resource name", kind)
	}
	if !raw {
		if _, err := Capability(kind); err != nil {
			return nil, err
		}
	}
	return &NativeNamespace{client: r.client, resourceType: kind, raw: raw}, nil
}

func ResourcePath(kind, method, id string, raw bool) (string, error) {
	registry := ResourceRegistry{}
	namespace, err := registry.Of(kind, raw)
	if err != nil {
		return "", err
	}
	return namespace.Path(method, id)
}

func (n *NativeNamespace) Path(method, id string) (string, error) {
	if id != "" && !resourceIDPattern.MatchString(id) {
		return "", fmt.Errorf("invalid resource ID %q", id)
	}
	item := id != ""
	if !n.raw {
		capability, err := Capability(n.resourceType)
		if err != nil {
			return "", err
		}
		if !capability.Supports(method, item) {
			return "", fmt.Errorf("%s is not documented for %s (item: %t); use --raw for an explicit firmware-specific request", method, n.resourceType, item)
		}
	}
	path := n.resourceType
	if item {
		path += "/" + id
	}
	return path, nil
}

func (n *NativeNamespace) List(ctx context.Context) (ResourceResponse, error) {
	return n.request(ctx, http.MethodGet, "", nil)
}

func (n *NativeNamespace) Get(ctx context.Context, id string) (ResourceResponse, error) {
	if id == "" {
		return ResourceResponse{}, fmt.Errorf("resource ID must not be empty")
	}
	return n.request(ctx, http.MethodGet, id, nil)
}

func (n *NativeNamespace) Write(ctx context.Context, method, id string, body json.RawMessage) (ResourceResponse, error) {
	switch method {
	case http.MethodPost:
		if id != "" {
			return ResourceResponse{}, fmt.Errorf("POST targets a collection")
		}
	case http.MethodPut, http.MethodDelete:
		if id == "" {
			return ResourceResponse{}, fmt.Errorf("%s requires a resource ID", method)
		}
	default:
		return ResourceResponse{}, fmt.Errorf("unsupported write method %q", method)
	}
	if method != http.MethodDelete {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(body, &object); err != nil || object == nil {
			return ResourceResponse{}, fmt.Errorf("request body must be a JSON object")
		}
	} else if len(body) != 0 {
		return ResourceResponse{}, fmt.Errorf("DELETE does not accept a request body")
	}
	return n.request(ctx, method, id, body)
}

func (n *NativeNamespace) request(ctx context.Context, method, id string, body json.RawMessage) (ResourceResponse, error) {
	var response ResourceResponse
	path, err := n.Path(method, id)
	if err != nil {
		return response, err
	}
	// A nil RawMessage in an interface marshals to null; reads and deletes must have no body.
	var payload any
	if len(body) != 0 {
		payload = body
	}
	err = n.client.request(ctx, method, path, payload, &response)
	return response, err
}

func (n *NativeNamespace) ResolveTarget(ctx context.Context, target string) (string, error) {
	if isUUID(target) {
		return strings.ToLower(target), nil
	}
	if normalizeName(target) == "" {
		return "", fmt.Errorf("target must be a UUID or a non-empty name")
	}
	response, err := n.List(ctx)
	if err != nil {
		return "", err
	}
	if err := response.Err(); err != nil {
		return "", err
	}
	resource, err := findNamedResource(response.Data, n.resourceType, target)
	if err != nil {
		return "", err
	}
	id := resource.GetID()
	if !resourceIDPattern.MatchString(id) {
		return "", fmt.Errorf("bridge returned an invalid resource ID %q", id)
	}
	return id, nil
}
