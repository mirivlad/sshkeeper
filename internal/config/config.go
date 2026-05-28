package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	SSH   SSHConfig   `toml:"ssh"`
	Vault VaultConfig `toml:"vault"`
	UI    UIConfig    `toml:"ui"`

	// resolved paths
	ConfigDir string `toml:"-"`
	DataDir   string `toml:"-"`
}

type SSHConfig struct {
	Binary            string `toml:"binary"`
	ConnectTimeoutSec int    `toml:"connect_timeout_seconds"`
	TestCommand       string `toml:"test_command"`
}

type VaultConfig struct {
	AutoLockMinutes int `toml:"auto_lock_minutes"`
}

type UIConfig struct {
	ShowSecurityHints bool `toml:"show_security_hints"`
}

func defaultConfig() *Config {
	return &Config{
		SSH: SSHConfig{
			Binary:            "/usr/bin/ssh",
			ConnectTimeoutSec: 10,
			TestCommand:       "echo SSHKEEPER_OK",
		},
		Vault: VaultConfig{
			AutoLockMinutes: 15,
		},
		UI: UIConfig{
			ShowSecurityHints: false,
		},
	}
}

func Load() (*Config, error) {
	cfg := defaultConfig()

	configDir, dataDir, err := resolveDirs(os.Getenv("XDG_CONFIG_HOME"), os.Getenv("XDG_DATA_HOME"))
	if err != nil {
		return nil, err
	}
	cfg.ConfigDir = configDir
	cfg.DataDir = dataDir

	// Ensure dirs exist
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, err
	}

	configFile := filepath.Join(configDir, "config.toml")

	// Write default config if not exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		f, err := os.OpenFile(configFile, os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		if err := toml.NewEncoder(f).Encode(cfg); err != nil {
			return nil, err
		}
	}

	// Parse existing config
	if _, err := toml.DecodeFile(configFile, cfg); err != nil {
		return nil, err
	}

	// Re-apply paths since toml decode might overwrite
	cfg.ConfigDir = configDir
	cfg.DataDir = dataDir

	return cfg, nil
}

func resolveDirs(configRoot, dataRoot string) (string, string, error) {
	if configRoot == "" || dataRoot == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", "", err
		}
		if configRoot == "" {
			configRoot = filepath.Join(home, ".config")
		}
		if dataRoot == "" {
			dataRoot = filepath.Join(home, ".local", "share")
		}
	}
	return filepath.Join(configRoot, "sshkeeper"), filepath.Join(dataRoot, "sshkeeper"), nil
}
