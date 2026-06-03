package model

import (
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
	ID              int64      `json:"id"`
	Alias           string     `json:"alias"`
	DisplayName     string     `json:"display_name"`
	Host            string     `json:"host"`
	Port            int        `json:"port"`
	User            string     `json:"user"`
	AuthMethod      AuthMethod `json:"auth_method"`
	IdentityFile    string     `json:"identity_file"`
	ProxyJump       string     `json:"proxy_jump"`
	Route           Route      `json:"route"`
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

type SecretType string

const (
	SecretSSHPassword   SecretType = "ssh_password"
	SecretKeyPassphrase SecretType = "key_passphrase"
	SecretSudoPassword  SecretType = "sudo_password"
	SecretCustom        SecretType = "custom_secret"
)

type Secret struct {
	ID    string     `json:"id"`
	Type  SecretType `json:"type"`
	Nonce []byte     `json:"nonce"`
	Data  []byte     `json:"data"`
}

type ForwardType string

const (
	ForwardLocal   ForwardType = "local"
	ForwardRemote  ForwardType = "remote"
	ForwardDynamic ForwardType = "dynamic"
)

type Forward struct {
	ID         int64       `json:"id"`
	ServerID   int64       `json:"server_id"`
	Type       ForwardType `json:"type"`
	LocalAddr  string      `json:"local_addr"`
	LocalPort  int         `json:"local_port"`
	RemoteAddr string      `json:"remote_addr"`
	RemotePort int         `json:"remote_port"`
}

type Tag struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// --- Route ---

// RouteHop represents a single jump host in a route.
// IsProfile: true = use Alias (references a sshkeeper profile), false = use Raw (literal address).
type RouteHop struct {
	Alias    string `json:"alias"`
	Raw      string `json:"raw"`
	IsProfile bool  `json:"is_profile"`
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
		if h.IsProfile {
			parts[i] = h.Alias
		} else {
			parts[i] = h.Raw
		}
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
		if h.IsProfile {
			names[i] = h.Alias
		} else {
			names[i] = h.Raw
		}
	}
	return strings.Join(names, " → ") + " → " + target
}

// HasProfileLinks returns true if any hop references a known profile.
func (r Route) HasProfileLinks() bool {
	for _, h := range r.Hops {
		if h.IsProfile {
			return true
		}
	}
	return false
}

type CommandTemplate struct {
	ID          int64  `json:"id"`
	ServerID    int64  `json:"server_id"`
	Name        string `json:"name"`
	Command     string `json:"command"`
	Description string `json:"description"`
}
