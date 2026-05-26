package ssh

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mirivlad/sshkeeper/internal/model"
)

// ImportFromSSHConfig parses ~/.ssh/config and returns server profiles
func ImportFromSSHConfig() ([]*model.Server, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(home, ".ssh", "config")
	f, err := os.Open(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("~/.ssh/config not found")
		}
		return nil, err
	}
	defer f.Close()

	var servers []*model.Server
	var current *model.Server

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		key := strings.ToLower(fields[0])
		value := strings.Join(fields[1:], " ")

		switch key {
		case "host":
			if current != nil && current.Host != "" {
				servers = append(servers, current)
			}
			// Skip wildcard hosts and patterns
			if strings.Contains(value, "*") || strings.Contains(value, "?") {
				current = nil
				continue
			}
			current = &model.Server{
				Alias:      value,
				Host:       value,
				Port:       22,
				User:       "",
				AuthMethod: model.AuthKey,
			}

		case "hostname":
			if current != nil {
				current.Host = value
			}

		case "port":
			if current != nil {
				if port, err := strconv.Atoi(value); err == nil {
					current.Port = port
				}
			}

		case "user":
			if current != nil {
				current.User = value
			}

		case "identityfile":
			if current != nil {
				current.IdentityFile = value
				if current.AuthMethod == model.AuthKey {
					current.AuthMethod = model.AuthKey
				}
			}

		case "proxyjump":
			if current != nil {
				current.ProxyJump = value
			}
		}
	}

	// Don't forget the last host
	if current != nil && current.Host != "" {
		servers = append(servers, current)
	}

	return servers, scanner.Err()
}
