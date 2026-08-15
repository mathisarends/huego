package hue

import (
	"context"
	"sort"
	"strings"
)

type named interface {
	GetID() string
	GetName() string
}

// resourceNamespace is the generic, stateless interface to one CLIP v2
// resource endpoint: GET .../<resourceType> lists, GET .../<resourceType>/<id>
// fetches one, and find-by-name resolves a human-typed name fuzzily.
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

// FindByName resolves a name to a resource: an exact case-insensitive match
// wins outright, otherwise the closest fuzzy match is used if it clears a
// similarity threshold, mirroring hueify's find_by_name.
func (r resourceNamespace[T]) FindByName(ctx context.Context, name string) (T, error) {
	var zero T
	resources, err := r.List(ctx)
	if err != nil {
		return zero, err
	}

	wanted := normalizeName(name)
	for _, res := range resources {
		if normalizeName(res.GetName()) == wanted {
			return res, nil
		}
	}

	sort.SliceStable(resources, func(i, j int) bool {
		si := similarity(wanted, resources[i].GetName())
		sj := similarity(wanted, resources[j].GetName())
		if si != sj {
			return si > sj
		}
		return normalizeName(resources[i].GetName()) < normalizeName(resources[j].GetName())
	})

	if len(resources) > 0 && similarity(wanted, resources[0].GetName()) >= fuzzyMatchCutoff {
		return resources[0], nil
	}

	names := make([]string, len(resources))
	for i, res := range resources {
		names[i] = res.GetName()
	}
	known := strings.Join(names, ", ")
	if known == "" {
		known = "none"
	}
	return zero, newResourceNotFoundError("no %s named %q. Known names: %s", r.resourceType, name, known)
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

const fuzzyMatchCutoff = 0.75

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// similarity is a Levenshtein-distance-based ratio in [0, 1], serving the
// same fuzzy-fallback role as Python's difflib.SequenceMatcher.ratio().
func similarity(a, b string) float64 {
	a, b = normalizeName(a), normalizeName(b)
	if a == b {
		return 1
	}
	distance := levenshtein(a, b)
	total := len(a) + len(b)
	if total == 0 {
		return 1
	}
	return 1 - float64(distance)/float64(total)*2
}

func levenshtein(a, b string) int {
	ar, br := []rune(a), []rune(b)
	prev := make([]int, len(br)+1)
	curr := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		curr[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(br)]
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
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}
