package config

import "path/filepath"

func DBPath(dataDir string) string {
	return filepath.Join(dataDir, "sshkeeper.db")
}

func VaultPath(dataDir string) string {
	return filepath.Join(dataDir, "vault.bin")
}
