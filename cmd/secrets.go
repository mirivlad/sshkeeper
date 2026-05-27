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

func serverSecretID(alias, secretType string) string {
	return fmt.Sprintf("server:%s:%s", alias, secretType)
}

func cleanupServerSecrets(v *vault.Vault, alias string) {
	for _, secretType := range serverSecretTypes {
		v.Delete(serverSecretID(alias, secretType))
	}
}

func syncServerSecrets(v *vault.Vault, oldAlias string, server *model.Server, secret string) error {
	if oldAlias == "" {
		oldAlias = server.Alias
	}
	if oldAlias != server.Alias {
		for _, secretType := range serverSecretTypes {
			oldID := serverSecretID(oldAlias, secretType)
			data, err := v.Get(oldID)
			if err == nil {
				if err := v.Put(serverSecretID(server.Alias, secretType), secretType, data); err != nil {
					return err
				}
			}
			v.Delete(oldID)
		}
	}

	switch server.AuthMethod {
	case model.AuthPassword:
		v.Delete(serverSecretID(server.Alias, secretKeyPassphrase))
		if secret != "" {
			return v.Put(serverSecretID(server.Alias, secretSSHPassword), secretSSHPassword, []byte(secret))
		}
	case model.AuthKeyPassphrase:
		v.Delete(serverSecretID(server.Alias, secretSSHPassword))
		if secret != "" {
			return v.Put(serverSecretID(server.Alias, secretKeyPassphrase), secretKeyPassphrase, []byte(secret))
		}
	default:
		v.Delete(serverSecretID(server.Alias, secretSSHPassword))
		v.Delete(serverSecretID(server.Alias, secretKeyPassphrase))
	}

	return nil
}

func deleteVaultSecrets(v *vault.Vault, alias string, secretType string) error {
	if secretType != "" {
		v.Delete(serverSecretID(alias, secretType))
		return nil
	}
	cleanupServerSecrets(v, alias)
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
