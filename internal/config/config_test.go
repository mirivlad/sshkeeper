package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveDirsUsesAppSubdirectoriesUnderXDGRoots(t *testing.T) {
	configRoot := filepath.Join(t.TempDir(), "config")
	dataRoot := filepath.Join(t.TempDir(), "data")

	configDir, dataDir, err := resolveDirs(configRoot, dataRoot)
	if err != nil {
		t.Fatalf("resolve dirs: %v", err)
	}

	if configDir != filepath.Join(configRoot, "sshkeeper") {
		t.Fatalf("config dir = %q; want app dir under XDG config root", configDir)
	}
	if dataDir != filepath.Join(dataRoot, "sshkeeper") {
		t.Fatalf("data dir = %q; want app dir under XDG data root", dataDir)
	}
}

func TestLanguagePreferencePersistsWithoutDiscardingConfig(t *testing.T) {
	configRoot := filepath.Join(t.TempDir(), "config")
	dataRoot := filepath.Join(t.TempDir(), "data")
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("XDG_DATA_HOME", dataRoot)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.Language != "auto" {
		t.Fatalf("default language = %q", cfg.UI.Language)
	}
	path := filepath.Join(cfg.ConfigDir, "config.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("\n[custom]\nvalue = 42 # keep me\n")...)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := cfg.SetLanguage("ru"); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.UI.Language != "ru" {
		t.Fatalf("reloaded language = %q", reloaded.UI.Language)
	}
	readOnlyLanguage, err := ReadLanguage()
	if err != nil || readOnlyLanguage != "ru" {
		t.Fatalf("ReadLanguage = %q, %v", readOnlyLanguage, err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "value = 42 # keep me") {
		t.Fatal("unrelated config was discarded")
	}
	if err := cfg.SetLanguage("de"); err == nil || cfg.UI.Language != "ru" {
		t.Fatal("invalid preference changed in-memory config")
	}
}

func TestSetLanguageAddsFieldToLegacyUISection(t *testing.T) {
	configRoot := filepath.Join(t.TempDir(), "config")
	dataRoot := filepath.Join(t.TempDir(), "data")
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("XDG_DATA_HOME", dataRoot)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cfg.ConfigDir, "config.toml")
	legacy := "[ui]\nshow_security_hints = true\n\n[custom]\nvalue = 42\n"
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	if err := cfg.SetLanguage("en"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "[ui]\nshow_security_hints = true\n\nlanguage = \"en\"\n[custom]") {
		t.Fatalf("language was not inserted in UI section:\n%s", data)
	}
	if value, err := ReadLanguage(); err != nil || value != "en" {
		t.Fatalf("ReadLanguage = %q, %v", value, err)
	}
}
