package cmd

import (
	"fmt"

	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/mirivlad/sshkeeper/internal/ssh"
	"github.com/mirivlad/sshkeeper/internal/vault"
)

const (
	secretSSHPassword   = "ssh_password"
	secretKeyPassphrase = "key_passphrase"
	secretSudoPassword  = "sudo_password"
)

var serverSecretTypes = []string{
	secretSSHPassword,
	secretKeyPassphrase,
	secretSudoPassword,
}

// serverSecretID is the legacy alias-based key kept for migration/tests.
func serverSecretID(alias, secretType string) string {
	return fmt.Sprintf("server:%s:%s", alias, secretType)
}

func stableServerSecretID(serverID int64, secretType string) string {
	return fmt.Sprintf("server-id:%d:%s", serverID, secretType)
}

func getServerSecret(v *vault.Vault, server *model.Server, secretType string) ([]byte, error) {
	if server == nil {
		return nil, fmt.Errorf("server is required")
	}
	if server.ID > 0 {
		stableID := stableServerSecretID(server.ID, secretType)
		if data, err := v.Get(stableID); err == nil {
			return data, nil
		}
	}
	legacyID := serverSecretID(server.Alias, secretType)
	data, err := v.Get(legacyID)
	if err != nil {
		return nil, err
	}
	if server.ID > 0 {
		if err := v.Put(stableServerSecretID(server.ID, secretType), secretType, data); err != nil {
			return nil, err
		}
		v.Delete(legacyID)
		if err := v.Save(); err != nil {
			return nil, fmt.Errorf("save migrated vault secret: %w", err)
		}
	}
	return data, nil
}

func hasServerSecret(v *vault.Vault, server *model.Server, secretType string) bool {
	if server == nil {
		return false
	}
	if server.ID > 0 && v.HasSecret(stableServerSecretID(server.ID, secretType)) {
		return true
	}
	return v.HasSecret(serverSecretID(server.Alias, secretType))
}

func cleanupServerSecretsForServer(v *vault.Vault, server *model.Server, legacyAliases ...string) {
	if server == nil {
		return
	}
	aliases := append([]string{server.Alias}, legacyAliases...)
	for _, secretType := range serverSecretTypes {
		if server.ID > 0 {
			v.Delete(stableServerSecretID(server.ID, secretType))
		}
		for _, alias := range aliases {
			if alias != "" {
				v.Delete(serverSecretID(alias, secretType))
			}
		}
	}
}

// syncServerSecrets writes credentials only under stable identity after the DB
// save has succeeded. Existing alias keys are migrated without depending on a
// rename operation, so a failed DB rename cannot orphan credentials.

// cleanupServerSecrets keeps the legacy helper surface for CLI/tests and also
// removes stable-ID records when the server still exists.
func cleanupServerSecrets(v *vault.Vault, alias string) {
	if appDB != nil {
		server, _ := appDB.GetServer(alias)
		if server != nil {
			cleanupServerSecretsForServer(v, server)
			return
		}
	}
	for _, secretType := range serverSecretTypes {
		v.Delete(serverSecretID(alias, secretType))
	}
}

func syncServerSecrets(v *vault.Vault, oldAlias string, server *model.Server, secret string) error {
	if server == nil {
		return fmt.Errorf("server is required")
	}
	if server.ID <= 0 {
		// Compatibility path for pre-persistence callers/tests. Real saves assign
		// Server.ID before this function is called. Keep old alias-based vaults
		// working and complete alias renames atomically in memory.
		if oldAlias != "" && oldAlias != server.Alias {
			for _, secretType := range serverSecretTypes {
				oldID := serverSecretID(oldAlias, secretType)
				if data, err := v.Get(oldID); err == nil {
					if err := v.Put(serverSecretID(server.Alias, secretType), secretType, data); err != nil {
						return err
					}
					v.Delete(oldID)
				}
			}
		}
		key := func(secretType string) string { return serverSecretID(server.Alias, secretType) }
		switch server.AuthMethod {
		case model.AuthPassword:
			v.Delete(key(secretKeyPassphrase))
			if secret != "" {
				return v.Put(key(secretSSHPassword), secretSSHPassword, []byte(secret))
			}
		case model.AuthKeyPassphrase:
			v.Delete(key(secretSSHPassword))
			if secret != "" {
				return v.Put(key(secretKeyPassphrase), secretKeyPassphrase, []byte(secret))
			}
		default:
			v.Delete(key(secretSSHPassword))
			v.Delete(key(secretKeyPassphrase))
		}
		return nil
	}

	aliases := []string{server.Alias}
	if oldAlias != "" && oldAlias != server.Alias {
		aliases = append(aliases, oldAlias)
	}
	for _, secretType := range serverSecretTypes {
		stableID := stableServerSecretID(server.ID, secretType)
		if !v.HasSecret(stableID) {
			for _, alias := range aliases {
				legacyID := serverSecretID(alias, secretType)
				if data, err := v.Get(legacyID); err == nil {
					if err := v.Put(stableID, secretType, data); err != nil {
						return err
					}
					break
				}
			}
		}
		for _, alias := range aliases {
			v.Delete(serverSecretID(alias, secretType))
		}
	}

	switch server.AuthMethod {
	case model.AuthPassword:
		v.Delete(stableServerSecretID(server.ID, secretKeyPassphrase))
		if secret != "" {
			return v.Put(stableServerSecretID(server.ID, secretSSHPassword), secretSSHPassword, []byte(secret))
		}
	case model.AuthKeyPassphrase:
		v.Delete(stableServerSecretID(server.ID, secretSSHPassword))
		if secret != "" {
			return v.Put(stableServerSecretID(server.ID, secretKeyPassphrase), secretKeyPassphrase, []byte(secret))
		}
	default:
		v.Delete(stableServerSecretID(server.ID, secretSSHPassword))
		v.Delete(stableServerSecretID(server.ID, secretKeyPassphrase))
	}
	return nil
}

func deleteVaultSecrets(v *vault.Vault, alias string, secretType string) error {
	var server *model.Server
	if appDB != nil {
		server, _ = appDB.GetServer(alias)
	}
	if server == nil {
		if secretType != "" {
			v.Delete(serverSecretID(alias, secretType))
		} else {
			for _, t := range serverSecretTypes {
				v.Delete(serverSecretID(alias, t))
			}
		}
		return nil
	}
	if secretType != "" {
		v.Delete(stableServerSecretID(server.ID, secretType))
		v.Delete(serverSecretID(server.Alias, secretType))
		return nil
	}
	cleanupServerSecretsForServer(v, server)
	return nil
}

func formTestVaultFunc(getVault ssh.VaultFunc, server *model.Server, formSecret string) ssh.VaultFunc {
	return func(serverAlias string, secretType string) (string, error) {
		if (secretType == secretSSHPassword || secretType == secretKeyPassphrase) && formSecret != "" {
			return formSecret, nil
		}
		return getVault(serverAlias, secretType)
	}
}

func vaultFuncForServer(v *vault.Vault, server *model.Server) ssh.VaultFunc {
	return func(_ string, secretType string) (string, error) {
		if !v.IsUnlocked() {
			return "", fmt.Errorf("%s", vaultLockedProcessMessage())
		}
		data, err := getServerSecret(v, server, secretType)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
}
