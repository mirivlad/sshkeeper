package cmd

import (
	"strings"
	"testing"
)

func TestFormatVaultSecretsListDoesNotExposeSecretValues(t *testing.T) {
	v := newUnlockedTestVault(t)
	mustPutSecret(t, v, "prod", "ssh_password", "super-secret")
	mustPutSecret(t, v, "stage", "key_passphrase", "also-secret")

	output, err := formatVaultSecretsList(v)
	if err != nil {
		t.Fatalf("format vault secrets list: %v", err)
	}

	for _, want := range []string{"prod", "ssh_password", "stage", "key_passphrase"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected output to contain %q\noutput:\n%s", want, output)
		}
	}
	for _, secretValue := range []string{"super-secret", "also-secret"} {
		if strings.Contains(output, secretValue) {
			t.Fatalf("expected output not to expose secret value %q\noutput:\n%s", secretValue, output)
		}
	}
}

func TestFormatVaultSecretsListHandlesEmptyVault(t *testing.T) {
	v := newUnlockedTestVault(t)

	output, err := formatVaultSecretsList(v)
	if err != nil {
		t.Fatalf("format empty vault secrets list: %v", err)
	}
	if !strings.Contains(output, "No secrets stored.") {
		t.Fatalf("expected empty output message, got:\n%s", output)
	}
}
