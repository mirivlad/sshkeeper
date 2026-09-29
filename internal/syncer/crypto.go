package syncer

import (
	"bytes"
	"compress/gzip"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

// KeySize is the size of the random sync key. The bundle is encrypted with it
// directly, so a stolen bundle offers nothing to guess.
const KeySize = 32

const (
	bundleFormat  = "sshkeeper-sync"
	bundleVersion = 1
	pairingFormat = "sshkeeper-pairing"
)

var (
	// ErrWrongKey means the bundle was sealed by a different sync space.
	ErrWrongKey = errors.New("the storage belongs to a different sync space")
	// ErrPairingExpired means the pairing code is older than its lifetime.
	ErrPairingExpired = errors.New("the pairing code has expired")
	// ErrPairingInvalid means the code or the master password is wrong.
	ErrPairingInvalid = errors.New("wrong pairing code or master password")
)

// NewKey returns a new random sync key.
func NewKey() ([]byte, error) {
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

// envelope is the only plaintext in the stored bundle: the format, a short
// key fingerprint to tell spaces apart, and the sealed payload.
type envelope struct {
	Format  string `json:"format"`
	Version int    `json:"version"`
	KeyID   string `json:"key_id"`
	Nonce   []byte `json:"nonce"`
	Data    []byte `json:"data"`
}

type payload struct {
	Records []Record `json:"records"`
}

// KeyID is a short public fingerprint of a sync key. It reveals nothing about
// the key and lets a device tell "wrong space" from "corrupted".
func KeyID(key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("sshkeeper-sync key id"))
	return fmt.Sprintf("%x", mac.Sum(nil)[:8])
}

// Seal encrypts records into a bundle with XChaCha20-Poly1305. The payload is
// compressed first; the envelope header is authenticated as associated data.
func Seal(key []byte, records []Record) ([]byte, error) {
	plain, err := json.Marshal(payload{Records: records})
	if err != nil {
		return nil, err
	}
	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	if _, err := zw.Write(plain); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	env := envelope{Format: bundleFormat, Version: bundleVersion, KeyID: KeyID(key), Nonce: make([]byte, aead.NonceSize())}
	if _, err := rand.Read(env.Nonce); err != nil {
		return nil, err
	}
	env.Data = aead.Seal(nil, env.Nonce, compressed.Bytes(), env.associatedData())
	return json.Marshal(env)
}

// Open decrypts a bundle sealed with key.
func Open(key []byte, bundle []byte) ([]Record, error) {
	var env envelope
	if err := json.Unmarshal(bundle, &env); err != nil || env.Format != bundleFormat {
		return nil, fmt.Errorf("not an sshkeeper sync bundle")
	}
	if env.Version != bundleVersion {
		return nil, fmt.Errorf("sync bundle version %d is not supported; update sshkeeper", env.Version)
	}
	if env.KeyID != KeyID(key) {
		return nil, ErrWrongKey
	}
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	if len(env.Nonce) != aead.NonceSize() {
		return nil, fmt.Errorf("sync bundle is corrupted")
	}
	compressed, err := aead.Open(nil, env.Nonce, env.Data, env.associatedData())
	if err != nil {
		return nil, fmt.Errorf("sync bundle is corrupted or was modified")
	}
	zr, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	plain, err := io.ReadAll(io.LimitReader(zr, 256<<20))
	if err != nil {
		return nil, err
	}
	var p payload
	if err := json.Unmarshal(plain, &p); err != nil {
		return nil, err
	}
	return p.Records, nil
}

func (e envelope) associatedData() []byte {
	return []byte(fmt.Sprintf("%s/%d/%s", e.Format, e.Version, e.KeyID))
}

// BundleKeyID reads the key fingerprint of a bundle without decrypting it.
func BundleKeyID(bundle []byte) string {
	var env envelope
	if json.Unmarshal(bundle, &env) != nil {
		return ""
	}
	return env.KeyID
}

var recoveryEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// RecoveryKey renders the sync key for safekeeping in groups of four
// characters. It is needed only when no joined device is left.
func RecoveryKey(key []byte) string {
	encoded := recoveryEncoding.EncodeToString(key)
	var groups []string
	for len(encoded) > 4 {
		groups = append(groups, encoded[:4])
		encoded = encoded[4:]
	}
	groups = append(groups, encoded)
	return strings.Join(groups, "-")
}

// ParseRecoveryKey reverses RecoveryKey, ignoring case, spaces, and dashes.
func ParseRecoveryKey(text string) ([]byte, error) {
	clean := strings.Map(func(r rune) rune {
		switch {
		case r == '-' || r == ' ':
			return -1
		case r >= 'a' && r <= 'z':
			return r - 'a' + 'A'
		}
		return r
	}, strings.TrimSpace(text))
	key, err := recoveryEncoding.DecodeString(clean)
	if err != nil || len(key) != KeySize {
		return nil, fmt.Errorf("not a valid recovery key")
	}
	return key, nil
}

// LooksLikeRecoveryKey tells a recovery key from a six-digit pairing code.
func LooksLikeRecoveryKey(text string) bool {
	return len(strings.TrimSpace(text)) > 12
}

// PairingLifetime is how long a pairing code stays valid.
const PairingLifetime = 10 * time.Minute

// Pairing key derivation cost. The code adds only 20 bits, so the master
// password carries the strength; Argon2id makes each guess expensive.
const (
	pairingTime    = 3
	pairingMemory  = 64 * 1024
	pairingThreads = 4
)

type pairingEnvelope struct {
	Format  string `json:"format"`
	Expires int64  `json:"expires"`
	Salt    []byte `json:"salt"`
	Nonce   []byte `json:"nonce"`
	Data    []byte `json:"data"`
}

// NewPairingCode returns six random digits.
func NewPairingCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// NormalizePairingCode strips spaces and dashes: "123 456" → "123456".
func NormalizePairingCode(code string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, code)
}

// SealPairing wraps the sync key for a new device. Opening it needs both the
// six-digit code and the master password of the device that shows the code,
// and it is refused after expires.
func SealPairing(key []byte, code, password string, expires time.Time) ([]byte, error) {
	env := pairingEnvelope{Format: pairingFormat, Expires: expires.Unix(), Salt: make([]byte, 16)}
	if _, err := rand.Read(env.Salt); err != nil {
		return nil, err
	}
	aead, err := chacha20poly1305.NewX(pairingKey(code, password, env.Salt))
	if err != nil {
		return nil, err
	}
	env.Nonce = make([]byte, aead.NonceSize())
	if _, err := rand.Read(env.Nonce); err != nil {
		return nil, err
	}
	env.Data = aead.Seal(nil, env.Nonce, key, pairingAD(env.Expires))
	return json.Marshal(env)
}

// OpenPairing recovers the sync key from a pairing blob.
func OpenPairing(blob []byte, code, password string, now time.Time) ([]byte, error) {
	var env pairingEnvelope
	if err := json.Unmarshal(blob, &env); err != nil || env.Format != pairingFormat {
		return nil, fmt.Errorf("no valid pairing request found")
	}
	if now.Unix() > env.Expires {
		return nil, ErrPairingExpired
	}
	aead, err := chacha20poly1305.NewX(pairingKey(NormalizePairingCode(code), password, env.Salt))
	if err != nil {
		return nil, err
	}
	if len(env.Nonce) != aead.NonceSize() {
		return nil, ErrPairingInvalid
	}
	key, err := aead.Open(nil, env.Nonce, env.Data, pairingAD(env.Expires))
	if err != nil || len(key) != KeySize {
		return nil, ErrPairingInvalid
	}
	return key, nil
}

// PairingExpired reports whether a pairing blob is past its lifetime, so a
// device can clean up codes nobody used.
func PairingExpired(blob []byte, now time.Time) bool {
	var env pairingEnvelope
	if json.Unmarshal(blob, &env) != nil {
		return true
	}
	return now.Unix() > env.Expires
}

func pairingKey(code, password string, salt []byte) []byte {
	secret := append([]byte(password), 0)
	secret = append(secret, []byte(code)...)
	return argon2.IDKey(secret, salt, pairingTime, pairingMemory, pairingThreads, chacha20poly1305.KeySize)
}

func pairingAD(expires int64) []byte {
	ad := []byte(pairingFormat + "/")
	return binary.BigEndian.AppendUint64(ad, uint64(expires))
}
