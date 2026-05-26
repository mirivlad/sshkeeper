package model

import "time"

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
	GroupName       string     `json:"group_name"`
	Notes           string     `json:"notes"`
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
	ID      string     `json:"id"`
	Type    SecretType `json:"type"`
	Nonce   []byte     `json:"nonce"`
	Data    []byte     `json:"data"`
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

type CommandTemplate struct {
	ID       int64  `json:"id"`
	ServerID int64  `json:"server_id"`
	Name     string `json:"name"`
	Command  string `json:"command"`
}
