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

func TestSortPreferencePersistsAndFallsBack(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(t.TempDir(), "data"))
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.Sort != SortByName {
		t.Fatalf("default sort = %q", cfg.UI.Sort)
	}
	if err := cfg.SetLanguage("ru"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.SetSort(SortByRecent); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.UI.Sort != SortByRecent || reloaded.UI.Language != "ru" {
		t.Fatalf("reloaded ui = %#v", reloaded.UI)
	}
	if err := cfg.SetSort("size"); err == nil || cfg.UI.Sort != SortByRecent {
		t.Fatal("invalid sort changed in-memory config")
	}

	path := filepath.Join(cfg.ConfigDir, "config.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), `sort = "recent"`, `sort = "size"`, 1))
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	fallback, err := Load()
	if err != nil {
		t.Fatalf("unknown sort must not block startup: %v", err)
	}
	if fallback.UI.Sort != SortByName {
		t.Fatalf("unknown sort should fall back to name, got %q", fallback.UI.Sort)
	}
}

func TestGlyphsDefaultToUnicodeAndAcceptASCII(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(t.TempDir(), "data"))
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.Glyphs != GlyphsUnicode {
		t.Fatalf("default glyphs = %q", cfg.UI.Glyphs)
	}
	path := filepath.Join(cfg.ConfigDir, "config.toml")
	for value, want := range map[string]string{"ascii": GlyphsASCII, "emoji": GlyphsUnicode} {
		if err := os.WriteFile(path, []byte("[ui]\nglyphs = \""+value+"\"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		reloaded, err := Load()
		if err != nil {
			t.Fatalf("glyphs %q must not block startup: %v", value, err)
		}
		if reloaded.UI.Glyphs != want {
			t.Fatalf("glyphs %q loaded as %q, want %q", value, reloaded.UI.Glyphs, want)
		}
	}
}

func TestSyncSettingsPersistBesideOtherSections(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(t.TempDir(), "data"))
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Sync.Mode != SyncOff || !cfg.Sync.Auto {
		t.Fatalf("default sync = %#v", cfg.Sync)
	}
	path := filepath.Join(cfg.ConfigDir, "config.toml")
	legacy := "[ui]\nlanguage = \"ru\" # keep\n"
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	want := SyncConfig{Mode: SyncFolder, Folder: `C:\Users\me\Sync "shared"`, GitBranch: "sshkeeper", Auto: false}
	if err := cfg.SetSync(want); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Sync != want {
		t.Fatalf("sync = %#v, want %#v", reloaded.Sync, want)
	}
	if reloaded.UI.Language != "ru" {
		t.Fatal("unrelated section lost")
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `language = "ru" # keep`) {
		t.Fatalf("comment lost:\n%s", data)
	}
	want.Mode = SyncGit
	want.GitURL = "git@example.org:me/keys.git"
	if err := cfg.SetSync(want); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if !strings.Contains(string(data), "[sync]\nmode = \"git\"\nfolder = ") {
		t.Fatalf("sync keys should be contiguous:\n%s", data)
	}
	if strings.Count(string(data), "[sync]") != 1 || strings.Count(string(data), "mode =") != 1 {
		t.Fatalf("second save duplicated keys:\n%s", data)
	}
}
