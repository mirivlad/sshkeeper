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
	Sync  SyncConfig  `toml:"sync"`

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
	// Sort is the server list order in the TUI: name, recent, or group.
	Sort string `toml:"sort"`
	// Glyphs selects TUI symbols: unicode (default) or ascii for terminals
	// and fonts without box drawing.
	Glyphs string `toml:"glyphs"`
}

// SyncConfig selects where the encrypted sync bundle is stored. The sync key
// itself lives in the vault, never in this file.
type SyncConfig struct {
	// Mode is off, folder, or git.
	Mode      string `toml:"mode"`
	Folder    string `toml:"folder"`
	GitURL    string `toml:"git_url"`
	GitBranch string `toml:"git_branch"`
	// Auto syncs when the TUI starts and shortly after local changes.
	Auto bool `toml:"auto"`
}

// Sync modes accepted by sync.mode.
const (
	SyncOff    = "off"
	SyncFolder = "folder"
	SyncGit    = "git"
)

// SetSync persists the [sync] section.
func (cfg *Config) SetSync(sync SyncConfig) error {
	if sync.Mode != SyncFolder && sync.Mode != SyncGit {
		sync.Mode = SyncOff
	}
	auto := "false"
	if sync.Auto {
		auto = "true"
	}
	if err := cfg.setValues("sync", [][2]string{
		{"mode", tomlString(sync.Mode)},
		{"folder", tomlString(strings.TrimSpace(sync.Folder))},
		{"git_url", tomlString(strings.TrimSpace(sync.GitURL))},
		{"git_branch", tomlString(strings.TrimSpace(sync.GitBranch))},
		{"auto", auto},
	}); err != nil {
		return err
	}
	cfg.Sync = sync
	return nil
}

// TUI symbol sets accepted by ui.glyphs.
const (
	GlyphsUnicode = "unicode"
	GlyphsASCII   = "ascii"
)

// Server list orders accepted by ui.sort.
const (
	SortByName   = "name"
	SortByRecent = "recent"
	SortByGroup  = "group"
)

// ValidSort reports whether value is a supported ui.sort order.
func ValidSort(value string) bool {
	return value == SortByName || value == SortByRecent || value == SortByGroup
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
		Sync: SyncConfig{Mode: SyncOff, Auto: true},
		UI: UIConfig{
			ShowSecurityHints: false,
			Language:          i18n.Auto,
			Sort:              SortByName,
			Glyphs:            GlyphsUnicode,
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
	if !ValidSort(cfg.UI.Sort) {
		// The order is a convenience; an unknown value must not block startup.
		cfg.UI.Sort = SortByName
	}
	if cfg.UI.Glyphs != GlyphsASCII {
		cfg.UI.Glyphs = GlyphsUnicode
	}
	if cfg.Sync.Mode != SyncFolder && cfg.Sync.Mode != SyncGit {
		cfg.Sync.Mode = SyncOff
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
	sectionPattern = regexp.MustCompile(`^\s*\[[^\]]+\]\s*(?:#.*)?$`)
)

// SetLanguage persists only ui.language, preserving unrelated config values
// and comments. The in-memory setting changes only after the file is replaced.
func (cfg *Config) SetLanguage(value string) error {
	if !i18n.ValidPreference(value) {
		return fmt.Errorf("unsupported ui.language %q (use auto, ru, or en)", value)
	}
	if err := cfg.setUIString("language", value); err != nil {
		return err
	}
	cfg.UI.Language = value
	return nil
}

// SetSort persists only ui.sort, like SetLanguage.
func (cfg *Config) SetSort(value string) error {
	if !ValidSort(value) {
		return fmt.Errorf("unsupported ui.sort %q (use name, recent, or group)", value)
	}
	if err := cfg.setUIString("sort", value); err != nil {
		return err
	}
	cfg.UI.Sort = value
	return nil
}

// setUIString writes key = "value" into the [ui] section.
func (cfg *Config) setUIString(key, value string) error {
	return cfg.setValues("ui", [][2]string{{key, tomlString(value)}})
}

// tomlString renders a TOML basic string.
func tomlString(value string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "\t", `\t`).Replace(value)
	return `"` + escaped + `"`
}

// setValues writes the given key = value lines (values already rendered as
// TOML) into a section, preserving unrelated values and comments, and
// replaces the file atomically. A missing section is appended.
func (cfg *Config) setValues(section string, entries [][2]string) error {
	headerPattern := regexp.MustCompile(`^\s*\[` + regexp.QuoteMeta(section) + `\]\s*(?:#.*)?$`)
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
	for _, entry := range entries {
		keyPattern := regexp.MustCompile(`^\s*` + regexp.QuoteMeta(entry[0]) + `\s*=`)
		sectionStart, sectionEnd, keyLine := -1, len(lines), -1
		for index, line := range lines {
			if headerPattern.MatchString(line) {
				sectionStart = index
				continue
			}
			if sectionStart >= 0 && sectionEnd == len(lines) {
				if sectionPattern.MatchString(line) {
					sectionEnd = index
				} else if keyPattern.MatchString(line) {
					keyLine = index
				}
			}
		}
		text := entry[0] + " = " + entry[1]
		switch {
		case keyLine >= 0:
			lines[keyLine] = text
		case sectionStart >= 0:
			lines = append(lines[:sectionEnd], append([]string{text}, lines[sectionEnd:]...)...)
		default:
			for len(lines) > 0 && lines[len(lines)-1] == "" {
				lines = lines[:len(lines)-1]
			}
			lines = append(lines, "", "["+section+"]", text)
		}
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
	return os.Rename(temp.Name(), path)
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
