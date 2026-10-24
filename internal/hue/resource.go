package hue

import (
	"context"
	"strings"
)

type named interface {
	GetID() string
	GetName() string
}

type resourceNamespace[T named] struct {
	client       *Client
	resourceType string
}

func (r resourceNamespace[T]) List(ctx context.Context) ([]T, error) {
	var resp APIResponse[T]
	if err := r.client.get(ctx, r.resourceType, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (r resourceNamespace[T]) GetOne(ctx context.Context, id string) (T, error) {
	var zero T
	var resp APIResponse[T]
	if err := r.client.get(ctx, r.resourceType+"/"+id, &resp); err != nil {
		return zero, err
	}
	if len(resp.Data) == 0 {
		return zero, newResourceNotFoundError("no %s with ID %s", r.resourceType, id)
	}
	return resp.Data[0], nil
}

func (r resourceNamespace[T]) FindByName(ctx context.Context, name string) (T, error) {
	var zero T
	resources, err := r.List(ctx)
	if err != nil {
		return zero, err
	}
	return matchName(r.resourceType, resources, name, T.GetName)
}

// matchName returns the single resource whose label equals name. Duplicate
// names are common (every room has a "Relax" scene), so more than one match
// is an error instead of a silent first pick.
func matchName[T named](resourceType string, resources []T, name string, label func(T) string) (T, error) {
	var zero T
	wanted := normalizeName(name)
	var matches []T
	for _, res := range resources {
		if normalizeName(label(res)) == wanted {
			matches = append(matches, res)
		}
	}

	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		names := make([]string, len(resources))
		for i, res := range resources {
			names[i] = label(res)
		}
		known := strings.Join(names, ", ")
		if known == "" {
			known = "none"
		}
		return zero, newResourceNotFoundError("no %s named %q. Known names: %s", resourceType, name, known)
	default:
		ids := make([]string, len(matches))
		for i, res := range matches {
			ids[i] = res.GetID()
		}
		return zero, &AmbiguousNameError{ResourceType: resourceType, Name: name, Candidates: ids}
	}
}

// ResolveTarget resolves a UUID or a name to a resource ID.
func (r resourceNamespace[T]) ResolveTarget(ctx context.Context, target string) (string, error) {
	if isUUID(target) {
		return target, nil
	}
	resource, err := r.FindByName(ctx, target)
	if err != nil {
		return "", err
	}
	return resource.GetID(), nil
}

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
			if !isHex {
				return false
			}
		}
	}
	return true
}
