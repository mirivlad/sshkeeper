package vault

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

const (
	currentVersion    = 1
	saltLen           = 32
	nonceLen          = 24
	keyLen            = 32
	verifierID        = "__sshkeeper_vault_verifier__"
	verifierPlaintext = "sshkeeper-vault-verifier-v1"
)

type KDFMeta struct {
	Name        string `json:"name"`
	MemoryKiB   int    `json:"memory_kib"`
	Iterations  int    `json:"iterations"`
	Parallelism int    `json:"parallelism"`
	Salt        string `json:"salt"`
}

type Record struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

type VaultFile struct {
	Version  int      `json:"version"`
	KDF      KDFMeta  `json:"kdf"`
	Verifier *Record  `json:"verifier,omitempty"`
	Records  []Record `json:"records"`
}

type Vault struct {
	mu        sync.Mutex
	path      string
	masterKey []byte
	records   map[string]secretRecord
	modified  bool
}

type secretRecord struct {
	secretType string
	plaintext  []byte
}

type SecretMeta struct {
	ID    string
	Alias string
	Type  string
}

func New(path string) *Vault {
	return &Vault{
		path:    path,
		records: make(map[string]secretRecord),
	}
}

// Exists checks if vault file exists and has content
func Exists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Size() > 0
}

// Create initializes a new vault with a master password
func Create(path string, masterPassword string) error {
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("generate salt: %w", err)
	}

	kdf := KDFMeta{
		Name:        "argon2id",
		MemoryKiB:   65536,
		Iterations:  3,
		Parallelism: 1,
		Salt:        base64.StdEncoding.EncodeToString(salt),
	}

	fmt.Print("Deriving key...")

	key := argon2.IDKey([]byte(masterPassword), salt, uint32(kdf.Iterations), uint32(kdf.MemoryKiB), uint8(kdf.Parallelism), keyLen)

	verifier, err := newVerifierRecord(key)
	if err != nil {
		return fmt.Errorf("create verifier: %w", err)
	}

	vf := VaultFile{
		Version:  currentVersion,
		KDF:      kdf,
		Verifier: &verifier,
		Records:  []Record{},
	}

	data, err := json.Marshal(vf)
	if err != nil {
		return fmt.Errorf("marshal vault: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("create vault file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write vault: %w", err)
	}

	// Clear key from memory
	for i := range key {
		key[i] = 0
	}

	fmt.Println(" done.")
	return nil
}

// Unlock decrypts the vault with master password
func (v *Vault) Unlock(masterPassword string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	fmt.Print("Unlocking vault...")

	data, err := os.ReadFile(v.path)
	if err != nil {
		return fmt.Errorf("read vault file: %w", err)
	}

	var vf VaultFile
	if err := json.Unmarshal(data, &vf); err != nil {
		return fmt.Errorf("parse vault: %w", err)
	}

	if vf.Version != currentVersion {
		return fmt.Errorf("unsupported vault version: %d", vf.Version)
	}

	salt, err := base64.StdEncoding.DecodeString(vf.KDF.Salt)
	if err != nil {
		return fmt.Errorf("decode salt: %w", err)
	}

	key := argon2.IDKey([]byte(masterPassword), salt, uint32(vf.KDF.Iterations), uint32(vf.KDF.MemoryKiB), uint8(vf.KDF.Parallelism), keyLen)

	if vf.Verifier != nil {
		if err := verifyRecord(key, *vf.Verifier); err != nil {
			return fmt.Errorf("invalid master password")
		}
	} else if len(vf.Records) > 0 {
		if _, err := decryptRecord(key, vf.Records[0]); err != nil {
			return fmt.Errorf("invalid master password")
		}
	} else {
		return fmt.Errorf("vault cannot verify master password; recreate empty vault")
	}

	v.masterKey = key
	v.records = make(map[string]secretRecord)

	for _, rec := range vf.Records {
		plaintext, err := decryptRecord(key, rec)
		if err != nil {
			return fmt.Errorf("decrypt record %s: %w", rec.ID, err)
		}
		v.records[rec.ID] = secretRecord{
			secretType: inferSecretType(rec.ID, rec.Type),
			plaintext:  plaintext,
		}
	}

	fmt.Println(" done.")
	return nil
}

// Lock clears the master key and records from memory
func (v *Vault) Lock() {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.masterKey != nil {
		for i := range v.masterKey {
			v.masterKey[i] = 0
		}
	}
	v.masterKey = nil
	v.records = make(map[string]secretRecord)
}

// IsUnlocked returns whether the vault is currently unlocked
func (v *Vault) IsUnlocked() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.masterKey != nil
}

// Put stores a secret in memory (not persisted until Save)
func (v *Vault) Put(id string, secretType string, plaintext []byte) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.masterKey == nil {
		return fmt.Errorf("vault is locked")
	}

	data := make([]byte, len(plaintext))
	copy(data, plaintext)
	v.records[id] = secretRecord{secretType: secretType, plaintext: data}
	v.modified = true
	return nil
}

// Get retrieves a secret
func (v *Vault) Get(id string) ([]byte, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.masterKey == nil {
		return nil, fmt.Errorf("vault is locked")
	}

	record, ok := v.records[id]
	if !ok {
		return nil, fmt.Errorf("secret not found: %s", id)
	}

	result := make([]byte, len(record.plaintext))
	copy(result, record.plaintext)
	return result, nil
}

func (v *Vault) HasSecret(id string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	_, ok := v.records[id]
	return ok
}

func (v *Vault) ListSecrets() ([]SecretMeta, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.masterKey == nil {
		return nil, fmt.Errorf("vault is locked")
	}

	metas := make([]SecretMeta, 0, len(v.records))
	for id, record := range v.records {
		alias, secretType, ok := parseServerSecretID(id)
		if !ok {
			continue
		}
		if record.secretType != "" {
			secretType = record.secretType
		}
		metas = append(metas, SecretMeta{
			ID:    id,
			Alias: alias,
			Type:  secretType,
		})
	}
	sort.Slice(metas, func(i, j int) bool {
		if metas[i].Alias == metas[j].Alias {
			return metas[i].Type < metas[j].Type
		}
		return metas[i].Alias < metas[j].Alias
	})
	return metas, nil
}

// Delete removes a secret
func (v *Vault) Delete(id string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.records, id)
	v.modified = true
}

// Save persists encrypted vault to disk
func (v *Vault) Save() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.masterKey == nil {
		return fmt.Errorf("vault is locked")
	}

	salt, err := base64.StdEncoding.DecodeString(v.getSalt())
	if err != nil {
		return err
	}

	kdf := KDFMeta{
		Name:        "argon2id",
		MemoryKiB:   65536,
		Iterations:  3,
		Parallelism: 1,
		Salt:        base64.StdEncoding.EncodeToString(salt),
	}

	fmt.Print("Deriving key...")

	var records []Record
	for id, record := range v.records {
		rec, err := encryptRecordWithType(v.masterKey, id, record.secretType, record.plaintext)
		if err != nil {
			return fmt.Errorf("encrypt record %s: %w", id, err)
		}
		records = append(records, rec)
	}

	verifier, err := newVerifierRecord(v.masterKey)
	if err != nil {
		return fmt.Errorf("create verifier: %w", err)
	}

	vf := VaultFile{
		Version:  currentVersion,
		KDF:      kdf,
		Verifier: &verifier,
		Records:  records,
	}

	data, err := json.Marshal(vf)
	if err != nil {
		return fmt.Errorf("marshal vault: %w", err)
	}

	tmpPath := v.path + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("create temp vault: %w", err)
	}

	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write vault: %w", err)
	}
	f.Close()

	if err := os.Rename(tmpPath, v.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename vault: %w", err)
	}

	v.modified = false
	return nil
}

// ChangePassword re-encrypts the vault with a new master password
func (v *Vault) ChangePassword(newPassword string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.masterKey == nil {
		return fmt.Errorf("vault is locked")
	}

	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("generate salt: %w", err)
	}

	newKey := argon2.IDKey([]byte(newPassword), salt, 3, 65536, 1, keyLen)

	kdf := KDFMeta{
		Name:        "argon2id",
		MemoryKiB:   65536,
		Iterations:  3,
		Parallelism: 1,
		Salt:        base64.StdEncoding.EncodeToString(salt),
	}

	fmt.Print("Deriving key...")

	var records []Record
	for id, record := range v.records {
		rec, err := encryptRecordWithType(newKey, id, record.secretType, record.plaintext)
		if err != nil {
			return fmt.Errorf("encrypt record: %w", err)
		}
		records = append(records, rec)
	}

	verifier, err := newVerifierRecord(newKey)
	if err != nil {
		return fmt.Errorf("create verifier: %w", err)
	}

	vf := VaultFile{
		Version:  currentVersion,
		KDF:      kdf,
		Verifier: &verifier,
		Records:  records,
	}

	data, err := json.Marshal(vf)
	if err != nil {
		return err
	}

	tmpPath := v.path + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}
	f.Close()

	if err := os.Rename(tmpPath, v.path); err != nil {
		os.Remove(tmpPath)
		return err
	}

	// Swap key
	for i := range v.masterKey {
		v.masterKey[i] = 0
	}
	v.masterKey = newKey

	return nil
}

// Helper to get salt from existing vault
func (v *Vault) getSalt() string {
	data, err := os.ReadFile(v.path)
	if err != nil {
		return ""
	}
	var vf VaultFile
	if err := json.Unmarshal(data, &vf); err != nil {
		return ""
	}
	return vf.KDF.Salt
}

func encryptRecord(key []byte, id string, plaintext []byte) (Record, error) {
	return encryptRecordWithType(key, id, "", plaintext)
}

func encryptRecordWithType(key []byte, id string, secretType string, plaintext []byte) (Record, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return Record{}, err
	}

	nonce := make([]byte, nonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return Record{}, err
	}

	ciphertext := aead.Seal(nil, nonce, plaintext, []byte(id))

	return Record{
		ID:         id,
		Type:       secretType,
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}, nil
}

func inferSecretType(id string, recordType string) string {
	if recordType != "" {
		return recordType
	}
	_, secretType, ok := parseServerSecretID(id)
	if !ok {
		return ""
	}
	return secretType
}

func parseServerSecretID(id string) (string, string, bool) {
	parts := strings.Split(id, ":")
	if len(parts) != 3 || parts[0] != "server" || parts[1] == "" || parts[2] == "" {
		return "", "", false
	}
	return parts[1], parts[2], true
}

func decryptRecord(key []byte, rec Record) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}

	nonce, err := base64.StdEncoding.DecodeString(rec.Nonce)
	if err != nil {
		return nil, fmt.Errorf("decode nonce: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(rec.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}

	plaintext, err := aead.Open(nil, nonce, ciphertext, []byte(rec.ID))
	if err != nil {
		return nil, fmt.Errorf("decrypt failed: %w", err)
	}

	return plaintext, nil
}

func newVerifierRecord(key []byte) (Record, error) {
	rec, err := encryptRecord(key, verifierID, []byte(verifierPlaintext))
	if err != nil {
		return Record{}, err
	}
	rec.Type = "verifier"
	return rec, nil
}

func verifyRecord(key []byte, rec Record) error {
	plaintext, err := decryptRecord(key, rec)
	if err != nil {
		return err
	}
	if !SecureCompare(string(plaintext), verifierPlaintext) {
		return fmt.Errorf("invalid verifier")
	}
	return nil
}

// VerifyPassword checks if a master password is correct without unlocking
func VerifyPassword(path string, masterPassword string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	var vf VaultFile
	if err := json.Unmarshal(data, &vf); err != nil {
		return false, err
	}

	salt, err := base64.StdEncoding.DecodeString(vf.KDF.Salt)
	if err != nil {
		return false, err
	}

	key := argon2.IDKey([]byte(masterPassword), salt, uint32(vf.KDF.Iterations), uint32(vf.KDF.MemoryKiB), uint8(vf.KDF.Parallelism), keyLen)
	defer func() {
		for i := range key {
			key[i] = 0
		}
	}()

	if vf.Verifier != nil {
		return verifyRecord(key, *vf.Verifier) == nil, nil
	}

	if len(vf.Records) == 0 {
		return false, nil
	}
	_, err = decryptRecord(key, vf.Records[0])
	return err == nil, nil
}

// Constant-time comparison to prevent timing attacks
func SecureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// Ensure time import is used
var _ time.Duration
