package model

import (
	"fmt"
	"strconv"
	"strings"
)

// IsSupportedAuthMethod reports whether method is a supported sshkeeper auth mode.
func IsSupportedAuthMethod(method AuthMethod) bool {
	switch method {
	case AuthPassword, AuthKey, AuthKeyPassphrase, AuthAgent:
		return true
	default:
		return false
	}
}

// ValidateServerBasics validates fields that do not require database access.
func ValidateServerBasics(s *Server) error {
	if s == nil {
		return fmt.Errorf("server is required")
	}
	if strings.TrimSpace(s.Alias) == "" {
		return fmt.Errorf("alias is required")
	}
	if strings.TrimSpace(s.Host) == "" {
		return fmt.Errorf("host is required")
	}
	if s.Port < 1 || s.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if s.AuthMethod == "" {
		s.AuthMethod = AuthKey
	}
	if !IsSupportedAuthMethod(s.AuthMethod) {
		return fmt.Errorf("unsupported auth method: %s", s.AuthMethod)
	}
	if (s.AuthMethod == AuthKey || s.AuthMethod == AuthKeyPassphrase) && strings.TrimSpace(s.IdentityFile) == "" {
		// OpenSSH may still find a default key, so this is intentionally allowed.
	}
	return ValidateRouteShape(s.ID, s.Route)
}

// ValidateRouteShape validates a route without resolving external references.
func ValidateRouteShape(targetID int64, route Route) error {
	seenProfiles := map[int64]bool{}
	seenRaw := map[string]bool{}
	for i, hop := range route.Hops {
		if hop.Profile() {
			if hop.ServerID <= 0 && strings.TrimSpace(hop.Alias) == "" {
				return fmt.Errorf("route hop %d has no profile reference", i+1)
			}
			if targetID > 0 && hop.ServerID == targetID {
				return fmt.Errorf("route cannot use the target server itself as a hop")
			}
			if hop.ServerID > 0 {
				if seenProfiles[hop.ServerID] {
					return fmt.Errorf("route contains duplicate profile hop %s", hop.DisplayName())
				}
				seenProfiles[hop.ServerID] = true
			}
			continue
		}
		raw := strings.TrimSpace(hop.Raw)
		if raw == "" {
			return fmt.Errorf("route hop %d is empty", i+1)
		}
		if seenRaw[raw] {
			return fmt.Errorf("route contains duplicate raw hop %q", raw)
		}
		seenRaw[raw] = true
	}
	return nil
}

// AliasResolver resolves an sshkeeper alias to a stable server ID.
type AliasResolver func(alias string) (int64, bool)

// ParseRouteSpec parses CLI/legacy route syntax into an explicit Route.
// profile:<alias> requires an existing sshkeeper profile; raw:<target> is always
// a literal OpenSSH target. Unprefixed entries are backward-compatible: an
// exact known alias becomes a profile reference, otherwise the entry is raw.
func ParseRouteSpec(input string, resolve AliasResolver) (Route, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return Route{}, nil
	}
	parts := strings.Split(input, ",")
	route := Route{Hops: make([]RouteHop, 0, len(parts))}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		switch {
		case strings.HasPrefix(part, "profile:"):
			alias := strings.TrimSpace(strings.TrimPrefix(part, "profile:"))
			if alias == "" {
				return Route{}, fmt.Errorf("empty profile route hop")
			}
			if resolve == nil {
				return Route{}, fmt.Errorf("cannot resolve profile route hop %q", alias)
			}
			id, ok := resolve(alias)
			if !ok || id <= 0 {
				return Route{}, fmt.Errorf("route profile not found: %s", alias)
			}
			route.Hops = append(route.Hops, RouteHop{ServerID: id, Alias: alias, IsProfile: true})
		case strings.HasPrefix(part, "raw:"):
			raw := strings.TrimSpace(strings.TrimPrefix(part, "raw:"))
			if raw == "" {
				return Route{}, fmt.Errorf("empty raw route hop")
			}
			route.Hops = append(route.Hops, RouteHop{Raw: raw})
		default:
			if resolve != nil {
				if id, ok := resolve(part); ok && id > 0 {
					route.Hops = append(route.Hops, RouteHop{ServerID: id, Alias: part, IsProfile: true})
					continue
				}
			}
			route.Hops = append(route.Hops, RouteHop{Raw: part})
		}
	}
	if err := ValidateRouteShape(0, route); err != nil {
		return Route{}, err
	}
	return route, nil
}

// FormatRouteSpec returns an unambiguous CLI representation.
func FormatRouteSpec(route Route) string {
	parts := make([]string, 0, len(route.Hops))
	for _, hop := range route.Hops {
		if hop.Profile() {
			name := hop.Alias
			if name == "" {
				name = strconv.FormatInt(hop.ServerID, 10)
			}
			parts = append(parts, "profile:"+name)
		} else {
			parts = append(parts, "raw:"+hop.Raw)
		}
	}
	return strings.Join(parts, ",")
}
