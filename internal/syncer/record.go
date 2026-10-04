// Package syncer keeps sshkeeper data in step across devices through an
// end-to-end encrypted bundle stored in a folder or a git repository.
//
// Everything that leaves the device — profiles, forwards, templates, secrets,
// and private keys — travels inside one sealed bundle. The storage never sees
// names, hosts, or even how many secrets exist, so it does not need to be
// trusted.
package syncer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// Record kinds. The record ID is the kind, a colon, and a stable key.
const (
	KindServer   = "server"
	KindForward  = "forward"
	KindGroup    = "group"
	KindTag      = "tag"
	KindTemplate = "template"
	KindSecret   = "secret"
	KindKey      = "key"
)

// Record is one synchronized item. Updated is the Unix time in nanoseconds of
// the change that produced this version; a deletion is a record with Deleted
// set and no data, kept so the deletion reaches every device.
type Record struct {
	ID      string          `json:"id"`
	Kind    string          `json:"kind"`
	Updated int64           `json:"updated"`
	Deleted bool            `json:"deleted,omitempty"`
	Hash    string          `json:"hash,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// ServerData is a profile as it travels between devices. Device-local facts
// (last connection, last test) stay on each device.
type ServerData struct {
	Alias          string    `json:"alias"`
	DisplayName    string    `json:"display_name,omitempty"`
	Host           string    `json:"host"`
	Port           int       `json:"port"`
	User           string    `json:"user,omitempty"`
	AuthMethod     string    `json:"auth_method"`
	IdentityFile   string    `json:"identity_file,omitempty"`
	Group          string    `json:"group,omitempty"`
	Notes          string    `json:"notes,omitempty"`
	StartupCommand string    `json:"startup_command,omitempty"`
	Tags           []string  `json:"tags,omitempty"`
	Route          []HopData `json:"route,omitempty"`
}

// HopData is one route hop: another synced profile or a raw target. Alias is
// kept so a hop to a profile missing on this device can fall back to it.
type HopData struct {
	Server string `json:"server,omitempty"`
	Alias  string `json:"alias,omitempty"`
	Raw    string `json:"raw,omitempty"`
}

// ForwardData is a saved port forward of a synced profile.
type ForwardData struct {
	Server      string `json:"server"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
	LocalAddr   string `json:"local_addr,omitempty"`
	LocalPort   int    `json:"local_port,omitempty"`
	RemoteAddr  string `json:"remote_addr,omitempty"`
	RemotePort  int    `json:"remote_port,omitempty"`
	Enabled     bool   `json:"enabled"`
}

// NameData is a group or tag.
type NameData struct {
	Name string `json:"name"`
}

// TemplateData is a global command template.
type TemplateData struct {
	Name        string `json:"name"`
	Command     string `json:"command"`
	Description string `json:"description,omitempty"`
}

// SecretData is a vault secret of a synced profile.
type SecretData struct {
	Server string `json:"server"`
	Type   string `json:"type"`
	Value  []byte `json:"value"`
}

// KeyData is a private key file referenced by a profile, and its public half
// when present. New bundles store Path in portable home-relative form, e.g.
// "~/.ssh/id_ed25519". Apply also accepts legacy absolute Linux/Windows paths.
type KeyData struct {
	Path    string `json:"path"`
	Private []byte `json:"private"`
	Public  []byte `json:"public,omitempty"`
}

// NewRecord builds a record with its content hash.
func NewRecord(kind, key string, data any) (Record, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return Record{}, err
	}
	sum := sha256.Sum256(raw)
	return Record{ID: kind + ":" + key, Kind: kind, Hash: hex.EncodeToString(sum[:]), Data: raw}, nil
}

// KindOf returns the kind part of a record ID.
func KindOf(id string) string {
	for index := 0; index < len(id); index++ {
		if id[index] == ':' {
			return id[:index]
		}
	}
	return id
}

// SortRecords orders records by ID so bundles are deterministic.
func SortRecords(records []Record) {
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
}
