package vault

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/argon2"
)

func TestNewEmptyVaultRejectsWrongPassword(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.bin")

	if err := Create(path, "correct horse"); err != nil {
		t.Fatalf("create vault: %v", err)
	}

	ok, err := VerifyPassword(path, "wrong horse")
	if err != nil {
		t.Fatalf("verify wrong password: %v", err)
	}
	if ok {
		t.Fatal("expected wrong password to be rejected for a new empty vault")
	}

	v := New(path)
	if err := v.Unlock("wrong horse"); err == nil {
		t.Fatal("expected unlock with wrong password to fail for a new empty vault")
	}
}

func TestNewEmptyVaultAcceptsCorrectPassword(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.bin")

	if err := Create(path, "correct horse"); err != nil {
		t.Fatalf("create vault: %v", err)
	}

	ok, err := VerifyPassword(path, "correct horse")
	if err != nil {
		t.Fatalf("verify correct password: %v", err)
	}
	if !ok {
		t.Fatal("expected correct password to be accepted for a new empty vault")
	}

	v := New(path)
	if err := v.Unlock("correct horse"); err != nil {
		t.Fatalf("unlock with correct password: %v", err)
	}
}

func TestLegacyVaultWithRecordsStillVerifiesByFirstRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.bin")
	salt := []byte("12345678901234567890123456789012")
	key := argon2.IDKey([]byte("correct horse"), salt, 3, 65536, 1, keyLen)

	rec, err := encryptRecord(key, "server:test:ssh_password", []byte("secret"))
	if err != nil {
		t.Fatalf("encrypt legacy record: %v", err)
	}

	data, err := json.Marshal(VaultFile{
		Version: currentVersion,
		KDF: KDFMeta{
			Name:        "argon2id",
			MemoryKiB:   65536,
			Iterations:  3,
			Parallelism: 1,
			Salt:        base64.StdEncoding.EncodeToString(salt),
		},
		Records: []Record{rec},
	})
	if err != nil {
		t.Fatalf("marshal legacy vault: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write legacy vault: %v", err)
	}

	ok, err := VerifyPassword(path, "correct horse")
	if err != nil {
		t.Fatalf("verify legacy vault: %v", err)
	}
	if !ok {
		t.Fatal("expected legacy vault with records to accept correct password")
	}

	ok, err = VerifyPassword(path, "wrong horse")
	if err != nil {
		t.Fatalf("verify legacy vault with wrong password: %v", err)
	}
	if ok {
		t.Fatal("expected legacy vault with records to reject wrong password")
	}
}

func TestLegacyVaultWithPreReductionKDFStillUnlocks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.bin")
	salt := []byte("12345678901234567890123456789012")
	key := argon2.IDKey([]byte("correct horse"), salt, 2, 1024, 1, keyLen)

	rec, err := encryptRecord(key, "server:test:ssh_password", []byte("secret"))
	if err != nil {
		t.Fatalf("encrypt legacy record: %v", err)
	}

	data, err := json.Marshal(VaultFile{
		Version: currentVersion,
		KDF: KDFMeta{
			Name:        "argon2id",
			MemoryKiB:   1,
			Iterations:  2,
			Parallelism: 1,
			Salt:        base64.StdEncoding.EncodeToString(salt),
		},
		Records: []Record{rec},
	})
	if err != nil {
		t.Fatalf("marshal legacy vault: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write legacy vault: %v", err)
	}

	ok, err := VerifyPassword(path, "correct horse")
	if err != nil {
		t.Fatalf("verify legacy vault: %v", err)
	}
	if !ok {
		t.Fatal("expected legacy vault using pre-reduction KDF to accept correct password")
	}

	v := New(path)
	if err := v.Unlock("correct horse"); err != nil {
		t.Fatalf("unlock legacy vault using pre-reduction KDF: %v", err)
	}

	secret, err := v.Get("server:test:ssh_password")
	if err != nil {
		t.Fatalf("get legacy secret: %v", err)
	}
	if string(secret) != "secret" {
		t.Fatalf("unexpected legacy secret: %q", secret)
	}
}

func TestLegacyEmptyVaultWithoutVerifierCannotUnlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.bin")
	salt := []byte("12345678901234567890123456789012")

	data, err := json.Marshal(VaultFile{
		Version: currentVersion,
		KDF: KDFMeta{
			Name:        "argon2id",
			MemoryKiB:   65536,
			Iterations:  3,
			Parallelism: 1,
			Salt:        base64.StdEncoding.EncodeToString(salt),
		},
		Records: []Record{},
	})
	if err != nil {
		t.Fatalf("marshal legacy empty vault: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write legacy empty vault: %v", err)
	}

	ok, err := VerifyPassword(path, "any password")
	if err != nil {
		t.Fatalf("verify legacy empty vault: %v", err)
	}
	if ok {
		t.Fatal("expected legacy empty vault without verifier to be unverifiable")
	}

	v := New(path)
	if err := v.Unlock("any password"); err == nil {
		t.Fatal("expected legacy empty vault without verifier to reject unlock")
	}
}

func TestListSecretsReturnsMetadataWithoutPlaintext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.bin")
	if err := Create(path, "master"); err != nil {
		t.Fatalf("create vault: %v", err)
	}
	v := New(path)
	if err := v.Unlock("master"); err != nil {
		t.Fatalf("unlock vault: %v", err)
	}
	if err := v.Put("server:prod:ssh_password", "ssh_password", []byte("secret-password")); err != nil {
		t.Fatalf("put password: %v", err)
	}
	if err := v.Put("server:prod:key_passphrase", "key_passphrase", []byte("secret-passphrase")); err != nil {
		t.Fatalf("put passphrase: %v", err)
	}

	metas, err := v.ListSecrets()
	if err != nil {
		t.Fatalf("list secrets: %v", err)
	}
	if len(metas) != 2 {
		t.Fatalf("expected 2 secret metadata records, got %d: %#v", len(metas), metas)
	}
	if metas[0].Alias != "prod" || metas[0].Type != "key_passphrase" {
		t.Fatalf("unexpected first metadata record: %#v", metas[0])
	}
	if metas[1].Alias != "prod" || metas[1].Type != "ssh_password" {
		t.Fatalf("unexpected second metadata record: %#v", metas[1])
	}
}

func TestListSecretsPreservesTypesAfterSaveAndUnlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.bin")
	if err := Create(path, "master"); err != nil {
		t.Fatalf("create vault: %v", err)
	}
	v := New(path)
	if err := v.Unlock("master"); err != nil {
		t.Fatalf("unlock vault: %v", err)
	}
	if err := v.Put("server:prod:ssh_password", "ssh_password", []byte("secret-password")); err != nil {
		t.Fatalf("put password: %v", err)
	}
	if err := v.Save(); err != nil {
		t.Fatalf("save vault: %v", err)
	}

	reopened := New(path)
	if err := reopened.Unlock("master"); err != nil {
		t.Fatalf("unlock reopened vault: %v", err)
	}
	metas, err := reopened.ListSecrets()
	if err != nil {
		t.Fatalf("list reopened secrets: %v", err)
	}
	if len(metas) != 1 {
		t.Fatalf("expected 1 secret metadata record, got %d: %#v", len(metas), metas)
	}
	if metas[0].ID != "server:prod:ssh_password" || metas[0].Alias != "prod" || metas[0].Type != "ssh_password" {
		t.Fatalf("unexpected metadata after reopen: %#v", metas[0])
	}
}

func TestHasSecretReportsPresenceWithoutReturningValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.bin")
	if err := Create(path, "master"); err != nil {
		t.Fatalf("create vault: %v", err)
	}
	v := New(path)
	if err := v.Unlock("master"); err != nil {
		t.Fatalf("unlock vault: %v", err)
	}
	if err := v.Put("server:prod:ssh_password", "ssh_password", []byte("secret-password")); err != nil {
		t.Fatalf("put password: %v", err)
	}

	if !v.HasSecret("server:prod:ssh_password") {
		t.Fatal("expected saved password to be reported present")
	}
	if v.HasSecret("server:prod:key_passphrase") {
		t.Fatal("expected missing passphrase to be reported absent")
	}
}
