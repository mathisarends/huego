package hue

import (
	"context"
	"fmt"
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
	if err := resp.Err(); err != nil {
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
	if err := resp.Err(); err != nil {
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

	return findNamedResource(resources, r.resourceType, name)
}

func findNamedResource[T named](resources []T, kind, name string) (T, error) {
	var zero T
	wanted := normalizeName(name)
	var matches []T
	var names []string
	for _, resource := range resources {
		if resource.GetName() == "" {
			continue
		}
		names = append(names, resource.GetName())
		if wanted != "" && normalizeName(resource.GetName()) == wanted {
			matches = append(matches, resource)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		ids := make([]string, len(matches))
		for i, match := range matches {
			ids[i] = match.GetID()
		}
		return zero, fmt.Errorf("ambiguous %s name %q; use a UUID: %s", kind, name, strings.Join(ids, ", "))
	}
	known := strings.Join(names, ", ")
	if known == "" {
		known = "none"
	}
	return zero, newResourceNotFoundError("no %s named %q. Known names: %s", kind, name, known)
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
