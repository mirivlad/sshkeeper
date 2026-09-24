package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/mirivlad/sshkeeper/internal/i18n"
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
	ShowSecurityHints bool   `toml:"show_security_hints"`
	Language          string `toml:"language"`
}

func defaultConfig() *Config {
	return &Config{
		SSH: SSHConfig{
			Binary:            defaultSSHBinary(),
			ConnectTimeoutSec: 10,
			TestCommand:       "echo SSHKEEPER_OK",
		},
		Vault: VaultConfig{
			AutoLockMinutes: 15,
		},
		UI: UIConfig{
			ShowSecurityHints: false,
			Language:          i18n.Auto,
		},
	}
}

func defaultSSHBinary() string {
	if runtime.GOOS == "windows" {
		return "ssh.exe"
	}
	return "/usr/bin/ssh"
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
	if !i18n.ValidPreference(cfg.UI.Language) {
		return nil, fmt.Errorf("unsupported ui.language %q (use auto, ru, or en)", cfg.UI.Language)
	}

	// Re-apply paths since toml decode might overwrite
	cfg.ConfigDir = configDir
	cfg.DataDir = dataDir

	return cfg, nil
}

// ReadLanguage reads the preference without creating configuration files. It
// is used before Cobra renders help, which does not initialize the app.
func ReadLanguage() (string, error) {
	configDir, _, err := resolveDirs(os.Getenv("XDG_CONFIG_HOME"), os.Getenv("XDG_DATA_HOME"))
	if err != nil {
		return "", err
	}
	path := filepath.Join(configDir, "config.toml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return i18n.Auto, nil
	} else if err != nil {
		return "", err
	}
	parsed := struct {
		UI struct {
			Language string `toml:"language"`
		} `toml:"ui"`
	}{}
	if _, err := toml.DecodeFile(path, &parsed); err != nil {
		return "", err
	}
	if parsed.UI.Language == "" {
		return i18n.Auto, nil
	}
	if !i18n.ValidPreference(parsed.UI.Language) {
		return "", fmt.Errorf("unsupported ui.language %q (use auto, ru, or en)", parsed.UI.Language)
	}
	return parsed.UI.Language, nil
}

var (
	uiSectionPattern = regexp.MustCompile(`^\s*\[ui\]\s*(?:#.*)?$`)
	sectionPattern   = regexp.MustCompile(`^\s*\[[^\]]+\]\s*(?:#.*)?$`)
	languagePattern  = regexp.MustCompile(`^\s*language\s*=`)
)

// SetLanguage persists only ui.language, preserving unrelated config values
// and comments. The in-memory setting changes only after the file is replaced.
func (cfg *Config) SetLanguage(value string) error {
	if !i18n.ValidPreference(value) {
		return fmt.Errorf("unsupported ui.language %q (use auto, ru, or en)", value)
	}
	path := filepath.Join(cfg.ConfigDir, "config.toml")
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	newline := "\n"
	if strings.Contains(string(original), "\r\n") {
		newline = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(string(original), "\r\n", "\n"), "\n")
	sectionStart, sectionEnd, languageLine := -1, len(lines), -1
	for index, line := range lines {
		if uiSectionPattern.MatchString(line) {
			sectionStart = index
			continue
		}
		if sectionStart >= 0 && sectionEnd == len(lines) {
			if sectionPattern.MatchString(line) {
				sectionEnd = index
			} else if languagePattern.MatchString(line) {
				languageLine = index
			}
		}
	}
	entry := `language = "` + value + `"`
	switch {
	case languageLine >= 0:
		lines[languageLine] = entry
	case sectionStart >= 0:
		lines = append(lines[:sectionEnd], append([]string{entry}, lines[sectionEnd:]...)...)
	default:
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
		lines = append(lines, "[ui]", entry)
	}
	updated := strings.Join(lines, newline)
	if !strings.HasSuffix(updated, newline) {
		updated += newline
	}
	temp, err := os.CreateTemp(cfg.ConfigDir, ".config-*.toml")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if err := temp.Chmod(info.Mode().Perm()); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.WriteString(updated); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		return err
	}
	cfg.UI.Language = value
	return nil
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
