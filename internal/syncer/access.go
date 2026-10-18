package syncer

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

// AccessFile is an optional password-protected copy of the random sync key.
// It is stored separately from sshkeeper.sync to stay compatible with older
// clients that rewrite the bundle without knowing about password recovery.
const AccessFile = "sshkeeper.access"

const (
	accessFormat      = "sshkeeper-sync-access"
	accessVersion     = 1
	accessMemoryKiB   = 128 * 1024
	accessIterations  = 3
	accessParallelism = 1
)

var (
	ErrAccessMissing  = errors.New("password recovery is not enabled for this sync storage; enable it on an already connected device or use a recovery key")
	ErrAccessPassword = errors.New("wrong synchronization master password")
	ErrAccessMismatch = errors.New("password recovery belongs to a different sync space")
)

// AccessTransport is implemented by the real folder and git transports.
// Keeping it separate from Transport preserves the legacy in-memory fakes.
type AccessTransport interface {
	ReadAccess() ([]byte, error)
	WriteAccess([]byte) error
}

type accessEnvelope struct {
	Format      string `json:"format"`
	Version     int    `json:"version"`
	KeyID       string `json:"key_id"`
	Salt        []byte `json:"salt"`
	Nonce       []byte `json:"nonce"`
	MemoryKiB   int    `json:"memory_kib"`
	Iterations  int    `json:"iterations"`
	Parallelism int    `json:"parallelism"`
	Data        []byte `json:"data"`
}

func (e accessEnvelope) context() []byte {
	return []byte(fmt.Sprintf("%s/%d/%s", e.Format, e.Version, e.KeyID))
}

// SealAccess encrypts the *random sync key*, not any profiles, using an
// independently salted and intentionally expensive Argon2id password key.
// A stolen access file enables offline guessing of the master password: this
// is an unavoidable tradeoff of password-only recovery.
func SealAccess(key []byte, password string) ([]byte, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("invalid sync key size")
	}
	if password == "" {
		return nil, fmt.Errorf("master password must not be empty")
	}
	e := accessEnvelope{
		Format: accessFormat, Version: accessVersion, KeyID: KeyID(key),
		Salt: make([]byte, 32), Nonce: make([]byte, chacha20poly1305.NonceSizeX),
		MemoryKiB: accessMemoryKiB, Iterations: accessIterations, Parallelism: accessParallelism,
	}
	if _, err := rand.Read(e.Salt); err != nil {
		return nil, err
	}
	if _, err := rand.Read(e.Nonce); err != nil {
		return nil, err
	}
	derived := argon2.IDKey([]byte(password), e.Salt, uint32(e.Iterations), uint32(e.MemoryKiB), uint8(e.Parallelism), KeySize)
	defer zeroAccessBytes(derived)
	aead, err := chacha20poly1305.NewX(derived)
	if err != nil {
		return nil, err
	}
	e.Data = aead.Seal(nil, e.Nonce, key, e.context())
	return json.Marshal(e)
}

func accessMetadata(blob []byte) (accessEnvelope, error) {
	var e accessEnvelope
	if len(blob) > 4096 || json.Unmarshal(blob, &e) != nil || e.Format != accessFormat || e.Version != accessVersion {
		return e, fmt.Errorf("invalid password recovery file")
	}
	if len(e.Salt) != 32 || len(e.Nonce) != chacha20poly1305.NonceSizeX ||
		e.MemoryKiB != accessMemoryKiB || e.Iterations != accessIterations ||
		e.Parallelism != accessParallelism || len(e.Data) != KeySize+16 {
		return e, fmt.Errorf("unsupported or damaged password recovery file")
	}
	return e, nil
}

// OpenAccess returns the random sync key if the supplied password is correct.
// All untrusted Argon2 parameters are checked before allocating memory.
func OpenAccess(blob []byte, password string) ([]byte, error) {
	e, err := accessMetadata(blob)
	if err != nil {
		return nil, err
	}
	if password == "" {
		return nil, ErrAccessPassword
	}
	derived := argon2.IDKey([]byte(password), e.Salt, uint32(e.Iterations), uint32(e.MemoryKiB), uint8(e.Parallelism), KeySize)
	defer zeroAccessBytes(derived)
	aead, err := chacha20poly1305.NewX(derived)
	if err != nil {
		return nil, err
	}
	key, err := aead.Open(nil, e.Nonce, e.Data, e.context())
	if err != nil {
		return nil, ErrAccessPassword
	}
	if len(key) != KeySize || KeyID(key) != e.KeyID {
		zeroAccessBytes(key)
		return nil, ErrAccessMismatch
	}
	return key, nil
}

func zeroAccessBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
