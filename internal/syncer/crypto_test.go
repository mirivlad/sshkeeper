package syncer

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSealHidesEverythingAndRoundTrips(t *testing.T) {
	key, _ := NewKey()
	server, _ := NewRecord(KindServer, "uuid-1", ServerData{Alias: "prod-db", Host: "db.internal.example", Port: 22})
	secret, _ := NewRecord(KindSecret, "uuid-1:ssh_password", SecretData{Server: "uuid-1", Type: "ssh_password", Value: []byte("hunter2")})
	bundle, err := Seal(key, []Record{server, secret})
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"prod-db", "db.internal", "hunter2", "ssh_password", "uuid-1", "server"} {
		if bytes.Contains(bundle, []byte(leak)) {
			t.Fatalf("bundle leaks %q in plaintext", leak)
		}
	}
	records, err := Open(key, bundle)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].Hash != server.Hash || records[1].Hash != secret.Hash {
		t.Fatalf("round trip lost data: %+v", records)
	}
}

func TestOpenRejectsWrongKeyAndTampering(t *testing.T) {
	key, _ := NewKey()
	other, _ := NewKey()
	bundle, _ := Seal(key, nil)
	if _, err := Open(other, bundle); !errors.Is(err, ErrWrongKey) {
		t.Fatalf("wrong key error = %v", err)
	}

	var env envelope
	_ = json.Unmarshal(bundle, &env)
	env.Data[len(env.Data)-1] ^= 1
	tampered, _ := json.Marshal(env)
	if _, err := Open(key, tampered); err == nil || !strings.Contains(err.Error(), "modified") {
		t.Fatalf("tampered bundle error = %v", err)
	}
}

func TestRecoveryKeyRoundTripsLeniently(t *testing.T) {
	key, _ := NewKey()
	text := RecoveryKey(key)
	if strings.Count(text, "-") != 12 {
		t.Fatalf("recovery key should be grouped: %s", text)
	}
	messy := strings.ToLower(strings.ReplaceAll(text, "-", " "))
	parsed, err := ParseRecoveryKey(messy)
	if err != nil || !bytes.Equal(parsed, key) {
		t.Fatalf("parse %q: %v", messy, err)
	}
	if !LooksLikeRecoveryKey(text) || LooksLikeRecoveryKey("123 456") {
		t.Fatal("recovery key and pairing code must be told apart")
	}
}

func TestPairingNeedsCodePasswordAndTime(t *testing.T) {
	key, _ := NewKey()
	code, err := NewPairingCode()
	if err != nil || len(code) != 6 {
		t.Fatalf("code = %q, %v", code, err)
	}
	now := time.Unix(1_800_000_000, 0)
	blob, err := SealPairing(key, code, "master pw", now.Add(PairingLifetime))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(blob, key) {
		t.Fatal("pairing blob contains the raw key")
	}
	spaced := code[:3] + " " + code[3:]
	got, err := OpenPairing(blob, spaced, "master pw", now.Add(time.Minute))
	if err != nil || !bytes.Equal(got, key) {
		t.Fatalf("valid pairing failed: %v", err)
	}
	if _, err := OpenPairing(blob, code, "wrong pw", now); !errors.Is(err, ErrPairingInvalid) {
		t.Fatalf("wrong password error = %v", err)
	}
	wrongCode := "000000"
	if code == wrongCode {
		wrongCode = "111111"
	}
	if _, err := OpenPairing(blob, wrongCode, "master pw", now); !errors.Is(err, ErrPairingInvalid) {
		t.Fatalf("wrong code error = %v", err)
	}
	if _, err := OpenPairing(blob, code, "master pw", now.Add(PairingLifetime+time.Second)); !errors.Is(err, ErrPairingExpired) {
		t.Fatalf("expired error = %v", err)
	}
	if !PairingExpired(blob, now.Add(time.Hour)) || PairingExpired(blob, now) {
		t.Fatal("PairingExpired is wrong")
	}
}

func TestOfflinePairingCodeIsHighEntropyAndNeedsNoPassword(t *testing.T) {
	key, _ := NewKey()
	code, err := NewOfflinePairingCode()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(code, "SKP1-") || !LooksLikeOfflinePairingCode(code) || LooksLikeRecoveryKey(code) {
		t.Fatalf("offline code classification failed: %q", code)
	}
	now := time.Unix(1_800_000_000, 0)
	blob, err := SealPairing(key, code, "", now.Add(OfflinePairingLifetime))
	if err != nil {
		t.Fatal(err)
	}
	messy := strings.ToLower(strings.ReplaceAll(code, "-", " "))
	got, err := OpenPairing(blob, messy, "", now.Add(23*time.Hour))
	if err != nil || !bytes.Equal(got, key) {
		t.Fatalf("offline pairing failed: %v", err)
	}
	if _, err := OpenPairing(blob, code, "", now.Add(OfflinePairingLifetime+time.Second)); !errors.Is(err, ErrPairingExpired) {
		t.Fatalf("expired offline code error = %v", err)
	}
}
