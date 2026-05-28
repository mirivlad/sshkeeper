package config

import (
	"path/filepath"
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
