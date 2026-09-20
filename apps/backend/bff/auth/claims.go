package auth

import (
	"slices"
	"strings"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// WHY: the literal key is tried before traversal because a namespaced claim
// carries dots in its own name — Auth0 issues "https://app.example.com/tenant_id"
// — while Keycloak nests roles under "realm_access.roles". Splitting first would
// break the former; never splitting would break the latter.
func claimValue(claims map[string]any, path string) (any, bool) {
	if value, ok := claims[path]; ok {
		return value, true
	}

	segments := strings.Split(path, ".")
	if len(segments) == 1 {
		return nil, false
	}

	var current any = claims
	for _, segment := range segments {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		if current, ok = object[segment]; !ok {
			return nil, false
		}
	}
	return current, true
}

func stringClaim(claims map[string]any, path string) (string, bool) {
	value, ok := claimValue(claims, path)
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	if !ok || text == "" {
		return "", false
	}
	return text, true
}

// permissionsFrom unites every declared claim, deduplicated and sorted. It
// accepts the two shapes an authorization server uses: a space-separated string
// (RFC 6749 §3.3) and an array of strings.
func permissionsFrom(claims map[string]any, paths []string) []ports.Permission {
	seen := make(map[string]struct{})
	for _, path := range paths {
		value, ok := claimValue(claims, path)
		if !ok {
			continue
		}
		for _, granted := range grantedIn(value) {
			seen[granted] = struct{}{}
		}
	}

	out := make([]ports.Permission, 0, len(seen))
	for granted := range seen {
		out = append(out, ports.Permission(granted))
	}
	slices.Sort(out)
	return out
}

func grantedIn(value any) []string {
	switch typed := value.(type) {
	case string:
		return strings.Fields(typed)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok && text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}
