package cmd

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/mirivlad/sshkeeper/internal/vault"
)

func newUnlockedTestVault(t *testing.T) *vault.Vault {
	t.Helper()
	path := filepath.Join(t.TempDir(), "vault.bin")
	if err := vault.Create(path, "master"); err != nil {
		t.Fatalf("create vault: %v", err)
	}
	v := vault.New(path)
	if err := v.Unlock("master"); err != nil {
		t.Fatalf("unlock vault: %v", err)
	}
	return v
}

func mustPutSecret(t *testing.T, v *vault.Vault, alias, secretType, value string) {
	t.Helper()
	if err := v.Put(serverSecretID(alias, secretType), secretType, []byte(value)); err != nil {
		t.Fatalf("put %s: %v", secretType, err)
	}
}

func mustGetSecret(t *testing.T, v *vault.Vault, alias, secretType string) string {
	t.Helper()
	data, err := v.Get(serverSecretID(alias, secretType))
	if err != nil {
		t.Fatalf("get %s: %v", secretType, err)
	}
	return string(data)
}

func assertSecretMissing(t *testing.T, v *vault.Vault, alias, secretType string) {
	t.Helper()
	if data, err := v.Get(serverSecretID(alias, secretType)); err == nil {
		t.Fatalf("expected %s for %s to be missing, got %q", secretType, alias, string(data))
	}
}

func TestCleanupServerSecretsRemovesAuthAndSudoSecrets(t *testing.T) {
	v := newUnlockedTestVault(t)
	mustPutSecret(t, v, "prod", "ssh_password", "password")
	mustPutSecret(t, v, "prod", "key_passphrase", "passphrase")
	mustPutSecret(t, v, "prod", "sudo_password", "sudo")

	cleanupServerSecrets(v, "prod")

	assertSecretMissing(t, v, "prod", "ssh_password")
	assertSecretMissing(t, v, "prod", "key_passphrase")
	assertSecretMissing(t, v, "prod", "sudo_password")
}

func TestSyncServerSecretsDeletesAuthSecretsForKeyAuth(t *testing.T) {
	v := newUnlockedTestVault(t)
	mustPutSecret(t, v, "prod", "ssh_password", "password")
	mustPutSecret(t, v, "prod", "key_passphrase", "passphrase")

	server := &model.Server{Alias: "prod", AuthMethod: model.AuthKey}
	if err := syncServerSecrets(v, "prod", server, ""); err != nil {
		t.Fatalf("sync secrets: %v", err)
	}

	assertSecretMissing(t, v, "prod", "ssh_password")
	assertSecretMissing(t, v, "prod", "key_passphrase")
}

func TestSyncServerSecretsKeepsExistingPasswordWhenEditPasswordIsBlank(t *testing.T) {
	v := newUnlockedTestVault(t)
	mustPutSecret(t, v, "prod", "ssh_password", "old-password")
	mustPutSecret(t, v, "prod", "key_passphrase", "old-passphrase")

	server := &model.Server{Alias: "prod", AuthMethod: model.AuthPassword}
	if err := syncServerSecrets(v, "prod", server, ""); err != nil {
		t.Fatalf("sync secrets: %v", err)
	}

	if got := mustGetSecret(t, v, "prod", "ssh_password"); got != "old-password" {
		t.Fatalf("expected password to remain, got %q", got)
	}
	assertSecretMissing(t, v, "prod", "key_passphrase")
}

func TestSyncServerSecretsRenamesSecretsBeforeApplyingAuthCleanup(t *testing.T) {
	v := newUnlockedTestVault(t)
	mustPutSecret(t, v, "old", "ssh_password", "password")
	mustPutSecret(t, v, "old", "key_passphrase", "passphrase")
	mustPutSecret(t, v, "old", "sudo_password", "sudo")

	server := &model.Server{Alias: "new", AuthMethod: model.AuthKeyPassphrase}
	if err := syncServerSecrets(v, "old", server, ""); err != nil {
		t.Fatalf("sync secrets: %v", err)
	}

	assertSecretMissing(t, v, "old", "ssh_password")
	assertSecretMissing(t, v, "old", "key_passphrase")
	assertSecretMissing(t, v, "old", "sudo_password")
	assertSecretMissing(t, v, "new", "ssh_password")
	if got := mustGetSecret(t, v, "new", "key_passphrase"); got != "passphrase" {
		t.Fatalf("expected key passphrase to move, got %q", got)
	}
	if got := mustGetSecret(t, v, "new", "sudo_password"); got != "sudo" {
		t.Fatalf("expected sudo password to move, got %q", got)
	}
}

func TestSyncServerSecretsStoresNewSecretForSelectedAuthMethod(t *testing.T) {
	v := newUnlockedTestVault(t)
	mustPutSecret(t, v, "prod", "key_passphrase", "old-passphrase")

	server := &model.Server{Alias: "prod", AuthMethod: model.AuthPassword}
	if err := syncServerSecrets(v, "prod", server, "new-password"); err != nil {
		t.Fatalf("sync secrets: %v", err)
	}

	if got := mustGetSecret(t, v, "prod", "ssh_password"); got != "new-password" {
		t.Fatalf("expected new password, got %q", got)
	}
	assertSecretMissing(t, v, "prod", "key_passphrase")
}

func TestDeleteVaultSecretsForAliasAndType(t *testing.T) {
	v := newUnlockedTestVault(t)
	mustPutSecret(t, v, "prod", "ssh_password", "password")
	mustPutSecret(t, v, "prod", "key_passphrase", "passphrase")

	if err := deleteVaultSecrets(v, "prod", "ssh_password"); err != nil {
		t.Fatalf("delete vault secret: %v", err)
	}

	assertSecretMissing(t, v, "prod", "ssh_password")
	if got := mustGetSecret(t, v, "prod", "key_passphrase"); got != "passphrase" {
		t.Fatalf("expected passphrase to remain, got %q", got)
	}
}

func TestDeleteVaultSecretsForAlias(t *testing.T) {
	v := newUnlockedTestVault(t)
	mustPutSecret(t, v, "prod", "ssh_password", "password")
	mustPutSecret(t, v, "prod", "key_passphrase", "passphrase")

	if err := deleteVaultSecrets(v, "prod", ""); err != nil {
		t.Fatalf("delete vault secrets: %v", err)
	}

	assertSecretMissing(t, v, "prod", "ssh_password")
	assertSecretMissing(t, v, "prod", "key_passphrase")
}

func TestFormTestVaultFuncUsesSavedSecretWhenFormSecretBlank(t *testing.T) {
	vaultFunc := formTestVaultFunc(func(serverAlias string, secretType string) (string, error) {
		if serverAlias != "prod" || secretType != "ssh_password" {
			return "", fmt.Errorf("unexpected secret lookup %s %s", serverAlias, secretType)
		}
		return "saved-password", nil
	}, &model.Server{Alias: "prod", AuthMethod: model.AuthPassword}, "")

	got, err := vaultFunc("prod", "ssh_password")
	if err != nil {
		t.Fatalf("get password: %v", err)
	}
	if got != "saved-password" {
		t.Fatalf("expected saved password, got %q", got)
	}
}

func TestFormTestVaultFuncUsesFormSecretWhenProvided(t *testing.T) {
	vaultFunc := formTestVaultFunc(func(serverAlias string, secretType string) (string, error) {
		return "", fmt.Errorf("saved secret should not be used")
	}, &model.Server{Alias: "prod", AuthMethod: model.AuthPassword}, "typed-password")

	got, err := vaultFunc("prod", "ssh_password")
	if err != nil {
		t.Fatalf("get password: %v", err)
	}
	if got != "typed-password" {
		t.Fatalf("expected typed password, got %q", got)
	}
}
