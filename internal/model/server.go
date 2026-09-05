package model

import (
	"fmt"
	"strings"
	"time"
)

type AuthMethod string

const (
	AuthPassword      AuthMethod = "password"
	AuthKey           AuthMethod = "key"
	AuthKeyPassphrase AuthMethod = "key_passphrase"
	AuthAgent         AuthMethod = "agent"
)

type TestStatus string

const (
	TestUnknown TestStatus = "unknown"
	TestOK      TestStatus = "ok"
	TestFailed  TestStatus = "failed"
)

type Server struct {
	ID           int64      `json:"id"`
	Alias        string     `json:"alias"`
	DisplayName  string     `json:"display_name"`
	Host         string     `json:"host"`
	Port         int        `json:"port"`
	User         string     `json:"user"`
	AuthMethod   AuthMethod `json:"auth_method"`
	IdentityFile string     `json:"identity_file"`
	// ProxyJump is a deprecated compatibility projection of Route.
	ProxyJump       string     `json:"proxy_jump"`
	Route           Route      `json:"route"`
	GroupID         int64      `json:"group_id"`
	GroupName       string     `json:"group_name"`
	Notes           string     `json:"notes"`
	StartupCommand  string     `json:"startup_command"`
	Tags            []string   `json:"tags"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	LastConnectedAt *time.Time `json:"last_connected_at"`
	LastTestAt      *time.Time `json:"last_test_at"`
	LastTestStatus  TestStatus `json:"last_test_status"`
	LastTestError   string     `json:"last_test_error"`
}

type ForwardType string

const (
	ForwardLocal   ForwardType = "local"
	ForwardRemote  ForwardType = "remote"
	ForwardDynamic ForwardType = "dynamic"
)

type Forward struct {
	ID          int64       `json:"id"`
	ServerID    int64       `json:"server_id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Type        ForwardType `json:"type"`
	LocalAddr   string      `json:"local_addr"`
	LocalPort   int         `json:"local_port"`
	RemoteAddr  string      `json:"remote_addr"`
	RemotePort  int         `json:"remote_port"`
	Enabled     bool        `json:"enabled"`
}

// ForwardHumanExplanation returns a human-readable explanation of the forward.
func (f *Forward) ForwardHumanExplanation(serverAlias string) string {
	switch f.Type {
	case ForwardLocal:
		return fmt.Sprintf("Port %s:%d on this machine will be forwarded through %s to %s:%d.",
			f.LocalAddr, f.LocalPort, serverAlias, f.RemoteAddr, f.RemotePort)
	case ForwardRemote:
		return fmt.Sprintf("Port %s:%d on %s will be forwarded to %s:%d on this machine.",
			f.RemoteAddr, f.RemotePort, serverAlias, f.LocalAddr, f.LocalPort)
	case ForwardDynamic:
		return fmt.Sprintf("SOCKS proxy on %s:%d will route traffic through %s.",
			f.LocalAddr, f.LocalPort, serverAlias)
	default:
		return fmt.Sprintf("Forward %s: %s:%d → %s:%d", f.Type, f.LocalAddr, f.LocalPort, f.RemoteAddr, f.RemotePort)
	}
}

// ForwardSSHArgs returns the OpenSSH arguments for this forward.
func (f *Forward) ForwardSSHArgs() []string {
	switch f.Type {
	case ForwardLocal:
		return []string{"-L", fmt.Sprintf("%s:%d:%s:%d", f.LocalAddr, f.LocalPort, f.RemoteAddr, f.RemotePort)}
	case ForwardRemote:
		return []string{"-R", fmt.Sprintf("%s:%d:%s:%d", f.RemoteAddr, f.RemotePort, f.LocalAddr, f.LocalPort)}
	case ForwardDynamic:
		return []string{"-D", fmt.Sprintf("%s:%d", f.LocalAddr, f.LocalPort)}
	default:
		return nil
	}
}

// ForwardListen returns the listen address:port string.
func (f *Forward) ForwardListen() string {
	switch f.Type {
	case ForwardLocal, ForwardDynamic:
		return fmt.Sprintf("%s:%d", f.LocalAddr, f.LocalPort)
	case ForwardRemote:
		return fmt.Sprintf("%s:%d", f.RemoteAddr, f.RemotePort)
	default:
		return ""
	}
}

// ForwardTarget returns the target address:port string.
func (f *Forward) ForwardTarget() string {
	switch f.Type {
	case ForwardLocal:
		return fmt.Sprintf("%s:%d", f.RemoteAddr, f.RemotePort)
	case ForwardRemote:
		return fmt.Sprintf("%s:%d", f.LocalAddr, f.LocalPort)
	case ForwardDynamic:
		return "SOCKS"
	default:
		return ""
	}
}

type Tag struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	ServerCount int    `json:"server_count,omitempty"`
}

type Group struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	ServerCount int    `json:"server_count,omitempty"`
}

// --- Route ---

// RouteHop is either a stable reference to another sshkeeper profile or a raw
// OpenSSH jump target. Alias is a display/backward-compatibility cache only;
// ServerID is the identity for profile hops.
type RouteHop struct {
	ServerID  int64  `json:"server_id,omitempty"`
	Alias     string `json:"alias,omitempty"`
	Raw       string `json:"raw,omitempty"`
	IsProfile bool   `json:"is_profile"`
}

func (h RouteHop) Profile() bool { return h.IsProfile || h.ServerID != 0 }

func (h RouteHop) DisplayName() string {
	if h.Profile() {
		if h.Alias != "" {
			return h.Alias
		}
		return fmt.Sprintf("profile#%d", h.ServerID)
	}
	return h.Raw
}

// Route represents the SSH jump route for a server.
// Mode is computed from Hops length: 0=direct, 1=via, 2+=chain
type Route struct {
	Hops []RouteHop `json:"hops"`
}

// RouteMode returns the computed route mode.
func (r Route) RouteMode() string {
	switch len(r.Hops) {
	case 0:
		return "direct"
	case 1:
		return "via"
	default:
		return "chain"
	}
}

// ProxyJumpString builds the -J argument value from hops.
func (r Route) ProxyJumpString() string {
	parts := make([]string, len(r.Hops))
	for i, h := range r.Hops {
		parts[i] = h.DisplayName()
	}
	return strings.Join(parts, ",")
}

// DisplaySummary returns a human-readable route summary.
// direct → target / bastion → target / bastion → dmz-gw → target
func (r Route) DisplaySummary(target string) string {
	if len(r.Hops) == 0 {
		return "direct → " + target
	}
	names := make([]string, len(r.Hops))
	for i, h := range r.Hops {
		names[i] = h.DisplayName()
	}
	return strings.Join(names, " → ") + " → " + target
}

// HasProfileLinks returns true if any hop references a known profile.
func (r Route) HasProfileLinks() bool {
	for _, h := range r.Hops {
		if h.Profile() {
			return true
		}
	}
	return false
}

type CommandTemplate struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Command     string `json:"command"`
	Description string `json:"description"`
}

// --- Tunnel ---

// TunnelState represents a running or stopped tunnel process.
type TunnelState struct {
	ID          int64     `json:"id"`
	ServerID    int64     `json:"server_id"`
	ServerAlias string    `json:"server_alias"`
	Name        string    `json:"name"`
	PID         int       `json:"pid"`
	ForwardIDs  []int64   `json:"forward_ids"`
	ConfigPath  string    `json:"config_path,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	LastError   string    `json:"last_error"`
}
