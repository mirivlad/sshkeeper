package ssh

import "github.com/mirivlad/sshkeeper/internal/model"

// BuildRouteArgs builds SSH arguments for a route.
// Returns -J flag with the full ProxyJump chain if route has hops.
func BuildRouteArgs(route model.Route) []string {
	if len(route.Hops) == 0 {
		return nil
	}
	return []string{"-J", route.ProxyJumpString()}
}
