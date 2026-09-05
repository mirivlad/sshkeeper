package cmd

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func listIdentityFiles() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	sshDir := filepath.Join(home, ".ssh")
	entries, err := os.ReadDir(sshDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var paths []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".pub") || lower == "config" || strings.HasPrefix(lower, "known_hosts") || lower == "authorized_keys" {
			continue
		}
		if strings.HasPrefix(name, "id_") || strings.HasSuffix(lower, ".pem") || strings.HasSuffix(lower, ".key") {
			paths = append(paths, filepath.Join(sshDir, name))
		}
	}
	sort.Strings(paths)
	return paths, nil
}
