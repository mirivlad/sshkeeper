package syncer

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mirivlad/sshkeeper/internal/db"
	"github.com/mirivlad/sshkeeper/internal/vault"
)

// VaultKeyID is where the sync key lives in the local vault. The key never
// leaves the device except inside a pairing blob or as a recovery key.
const VaultKeyID = "sync:key"

const vaultKeyType = "sync_key"

// Meta keys in the local database.
const (
	metaLastSync  = "last_sync"
	metaLastError = "last_error"
)

var (
	// ErrNoKey means this device has not created or joined a sync space.
	ErrNoKey = errors.New("this device is not set up for sync")
	// ErrSpaceExists means the storage already holds a sync space, so this
	// device must join it instead of creating a new one.
	ErrSpaceExists = errors.New("the storage already has synced data; add this device with a pairing code from another device")
	// ErrVaultLocked means secrets are unavailable. Syncing without them
	// would look like every secret was deleted.
	ErrVaultLocked = errors.New("unlock the vault before syncing")
	// ErrNoPairing means no pairing code is waiting in the storage.
	ErrNoPairing = errors.New("no pairing code is waiting; create one on the other device first")
)

// Report summarizes one sync.
type Report struct {
	// Received counts changes applied from other devices.
	Received int
	// Sent counts local changes stored for other devices.
	Sent int
	// Stored reports whether the bundle was written.
	Stored   bool
	Warnings []string
	At       time.Time
}

// Engine runs sync for this device.
type Engine struct {
	DB        *db.DB
	Vault     *vault.Vault
	Transport Transport
	Home      string
	Now       func() time.Time
}

// engineLock keeps one sync at a time per process: the TUI may start one in
// the background while the user asks for another.
var engineLock sync.Mutex

func (e *Engine) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

func (e *Engine) local() *Local {
	return &Local{DB: e.DB, Vault: e.Vault, Home: e.Home}
}

// Key returns the sync key from the vault.
func (e *Engine) Key() ([]byte, error) {
	if !e.Vault.IsUnlocked() {
		return nil, ErrVaultLocked
	}
	key, err := e.Vault.Get(VaultKeyID)
	if err != nil || len(key) != KeySize {
		return nil, ErrNoKey
	}
	return key, nil
}

// HasKey reports whether this device has joined a sync space.
func (e *Engine) HasKey() bool {
	return e.Vault.IsUnlocked() && e.Vault.HasSecret(VaultKeyID)
}

// Create starts a new sync space from this device and returns its recovery
// key. It refuses when the storage already holds a space.
func (e *Engine) Create() (string, Report, error) {
	if !e.Vault.IsUnlocked() {
		return "", Report{}, ErrVaultLocked
	}
	bundles, err := e.Transport.Fetch()
	if err != nil {
		return "", Report{}, err
	}
	if len(bundles) > 0 {
		return "", Report{}, ErrSpaceExists
	}
	key, err := NewKey()
	if err != nil {
		return "", Report{}, err
	}
	if err := e.adoptKey(key); err != nil {
		return "", Report{}, err
	}
	report, err := e.Sync()
	return RecoveryKey(key), report, err
}

// Pair publishes the sync key for a new device, protected by a fresh
// six-digit code and the given master password. The caller must verify the
// password first.
func (e *Engine) Pair(password string) (string, time.Time, error) {
	key, err := e.Key()
	if err != nil {
		return "", time.Time{}, err
	}
	code, err := NewPairingCode()
	if err != nil {
		return "", time.Time{}, err
	}
	expires := e.now().Add(PairingLifetime)
	blob, err := SealPairing(key, code, password, expires)
	if err != nil {
		return "", time.Time{}, err
	}
	if err := e.Transport.WritePairing(blob); err != nil {
		return "", time.Time{}, err
	}
	return code, expires, nil
}

// PairOffline publishes a high-entropy one-time code that remains valid long
// enough to reboot into another OS. Unlike the six-digit code it does not need
// the old device's master password because the code itself has 128 random bits.
func (e *Engine) PairOffline() (string, time.Time, error) {
	key, err := e.Key()
	if err != nil {
		return "", time.Time{}, err
	}
	code, err := NewOfflinePairingCode()
	if err != nil {
		return "", time.Time{}, err
	}
	expires := e.now().Add(OfflinePairingLifetime)
	blob, err := SealPairing(key, code, "", expires)
	if err != nil {
		return "", time.Time{}, err
	}
	if err := e.Transport.WritePairing(blob); err != nil {
		return "", time.Time{}, err
	}
	return code, expires, nil
}

// Join adds this device to the space in the storage. secret is a short
// six-digit code plus the old device's master password, a long-lived SKP1
// offline code, or a recovery key.
func (e *Engine) Join(secret, password string) (Report, error) {
	if !e.Vault.IsUnlocked() {
		return Report{}, ErrVaultLocked
	}
	var key []byte
	offlinePairing := LooksLikeOfflinePairingCode(secret)
	pairing := offlinePairing || !LooksLikeRecoveryKey(secret)
	if pairing {
		blob, err := e.Transport.ReadPairing()
		if err != nil {
			return Report{}, err
		}
		if blob == nil {
			return Report{}, ErrNoPairing
		}
		pairingPassword := password
		if offlinePairing {
			pairingPassword = ""
		}
		if key, err = OpenPairing(blob, secret, pairingPassword, e.now()); err != nil {
			return Report{}, err
		}
	} else {
		var err error
		if key, err = ParseRecoveryKey(secret); err != nil {
			return Report{}, err
		}
	}
	bundles, err := e.Transport.Fetch()
	if err != nil {
		return Report{}, err
	}
	for _, bundle := range bundles {
		if BundleKeyID(bundle) != KeyID(key) {
			return Report{}, ErrWrongKey
		}
	}
	if err := e.adoptKey(key); err != nil {
		return Report{}, err
	}
	report, err := e.Sync()
	if err == nil && pairing {
		// The code has done its job; nobody else should find it.
		_ = e.Transport.DeletePairing()
	}
	return report, err
}

// RecoveryKey returns this device's recovery key.
func (e *Engine) RecoveryKey() (string, error) {
	key, err := e.Key()
	if err != nil {
		return "", err
	}
	return RecoveryKey(key), nil
}

// Leave removes the sync key and forgets the sync state. Local data stays.
func (e *Engine) Leave() error {
	if !e.Vault.IsUnlocked() {
		return ErrVaultLocked
	}
	e.Vault.Delete(VaultKeyID)
	if err := e.Vault.Save(); err != nil {
		return err
	}
	return e.DB.ClearSyncState()
}

func (e *Engine) adoptKey(key []byte) error {
	if err := e.Vault.Put(VaultKeyID, vaultKeyType, key); err != nil {
		return err
	}
	if err := e.Vault.Save(); err != nil {
		return err
	}
	// A new space starts from a clean slate: nothing was synced into it yet.
	return e.DB.ClearSyncState()
}

// Sync exchanges changes with the storage: fetch, merge, apply what came
// from other devices, and store the merged bundle when it changed.
func (e *Engine) Sync() (Report, error) {
	engineLock.Lock()
	defer engineLock.Unlock()
	report, err := e.syncOnce()
	for attempt := 0; errors.Is(err, ErrStorageChanged) && attempt < 3; attempt++ {
		report, err = e.syncOnce()
	}
	if err != nil {
		_ = e.DB.SetSyncMeta(metaLastError, err.Error())
		return report, err
	}
	_ = e.DB.SetSyncMeta(metaLastError, "")
	_ = e.DB.SetSyncMeta(metaLastSync, strconv.FormatInt(report.At.Unix(), 10))
	return report, nil
}

func (e *Engine) syncOnce() (Report, error) {
	key, err := e.Key()
	if err != nil {
		return Report{}, err
	}
	now := e.now()
	report := Report{At: now}

	bundles, err := e.Transport.Fetch()
	if err != nil {
		return report, err
	}
	var remote []Record
	for _, bundle := range bundles {
		records, err := Open(key, bundle)
		if err != nil {
			return report, err
		}
		// Several copies (folder conflict files) merge like two devices.
		remote = Merge(remote, records).Records
	}
	remoteByID := map[string]Record{}
	for _, record := range remote {
		remoteByID[record.ID] = record
	}

	states, err := e.loadStates()
	if err != nil {
		return report, err
	}
	local := e.local()
	if err := local.Adopt(remote, states); err != nil {
		return report, err
	}
	// Repair local-only cross-platform key paths before Export/Merge. This must
	// run even on a completely quiet sync where no remote record is Incoming.
	repaired, repairWarnings, err := local.RepairPortableKeyPaths(remote)
	if err != nil {
		return report, err
	}
	report.Received += repaired
	report.Warnings = append(report.Warnings, repairWarnings...)

	current, warnings, err := local.Export()
	if err != nil {
		return report, err
	}
	report.Warnings = append(report.Warnings, warnings...)

	stamped := Stamp(current, states, remoteByID, now.UnixNano())
	stamped = keepUnreadable(stamped, local.unreadable, remoteByID)
	result := Merge(stamped, remote)
	incoming := retryUnreadableRecords(local, result.Incoming, result.Records)

	applied, applyWarnings, err := local.Apply(incoming)
	report.Received += applied
	report.Warnings = append(report.Warnings, applyWarnings...)
	if err != nil {
		return report, err
	}

	if len(bundles) != 1 || !sameRecords(result.Records, remote) {
		bundle, err := Seal(key, result.Records)
		if err != nil {
			return report, err
		}
		if err := e.Transport.Store(bundle); err != nil {
			return report, err
		}
		report.Stored = true
		report.Sent = result.Outgoing
	}

	e.cleanupPairing(now)
	return report, e.saveStates(result.Records)
}

// saveStates remembers the merged state. When an incoming change could not be
// applied (a different key file or a clashing alias was kept), the local
// version is recorded as strictly older than the merged one. It then loses
// every later merge, so a device's own key or profile never travels to other
// devices by winning a tie, and the change is retried next time. Only a new
// local edit makes it newer again.
func (e *Engine) saveStates(merged []Record) error {
	current, _, err := e.local().Export()
	if err != nil {
		return err
	}
	localHash := map[string]string{}
	for _, record := range current {
		localHash[record.ID] = record.Hash
	}
	states := make([]db.SyncState, 0, len(merged))
	for _, record := range merged {
		state := db.SyncState{ID: record.ID, Hash: record.Hash, Updated: record.Updated, Deleted: record.Deleted}
		if !record.Deleted {
			if hash, ok := localHash[record.ID]; ok && hash != record.Hash {
				state.Hash = hash
				state.Updated = record.Updated - 1
			}
		}
		states = append(states, state)
	}
	return e.DB.ReplaceSyncStates(states)
}

func (e *Engine) loadStates() (map[string]Record, error) {
	states, err := e.DB.LoadSyncStates()
	if err != nil {
		return nil, err
	}
	records := make(map[string]Record, len(states))
	for id, state := range states {
		records[id] = Record{ID: id, Kind: KindOf(id), Hash: state.Hash, Updated: state.Updated, Deleted: state.Deleted}
	}
	return records, nil
}

func (e *Engine) cleanupPairing(now time.Time) {
	blob, err := e.Transport.ReadPairing()
	if err == nil && blob != nil && PairingExpired(blob, now) {
		_ = e.Transport.DeletePairing()
	}
}

// keepUnreadable replaces deletions of items that exist but could not be read
// with the remote copy, so a permission problem never deletes a key elsewhere.
func keepUnreadable(stamped []Record, unreadable map[string]bool, remote map[string]Record) []Record {
	if len(unreadable) == 0 {
		return stamped
	}
	kept := stamped[:0]
	for _, record := range stamped {
		if record.Deleted && unreadable[record.ID] {
			if copy, ok := remote[record.ID]; ok {
				kept = append(kept, copy)
			}
			continue
		}
		kept = append(kept, record)
	}
	return kept
}

// retryUnreadableRecords re-applies remote key material that this device is
// supposed to have but cannot currently read locally. This matters when
// upgrading a device that previously synchronized an absolute key path from a
// different OS: the record may be the same version as last time, so Merge
// would normally omit it from Incoming even though the local file is missing.
//
// Profiles that reference a retried key are also re-applied so IdentityFile is
// rewritten to the receiving OS's local path in the same sync.
func retryUnreadableRecords(local *Local, incoming, merged []Record) []Record {
	if local == nil || len(local.unreadable) == 0 {
		return incoming
	}

	seen := make(map[string]bool, len(incoming))
	for _, record := range incoming {
		seen[record.ID] = true
	}
	retryPaths := map[string]bool{}

	for _, record := range merged {
		if record.Kind != KindKey || record.Deleted || !local.unreadable[record.ID] {
			continue
		}
		if !seen[record.ID] {
			incoming = append(incoming, record)
			seen[record.ID] = true
		}
		var data KeyData
		if decode(record, &data) == nil {
			source := strings.TrimSpace(data.Path)
			if source != "" {
				retryPaths[source] = true
				retryPaths[local.portableKeyPath(source)] = true
			}
		}
	}

	if len(retryPaths) == 0 {
		SortRecords(incoming)
		return incoming
	}
	for _, record := range merged {
		if record.Kind != KindServer || record.Deleted || seen[record.ID] {
			continue
		}
		var data ServerData
		if decode(record, &data) != nil {
			continue
		}
		identity := strings.TrimSpace(data.IdentityFile)
		if identity == "" {
			continue
		}
		if retryPaths[identity] || retryPaths[local.portableKeyPath(identity)] {
			incoming = append(incoming, record)
			seen[record.ID] = true
		}
	}
	SortRecords(incoming)
	return incoming
}

func sameRecords(a, b []Record) bool {
	if len(a) != len(b) {
		return false
	}
	byID := map[string]Record{}
	for _, record := range b {
		byID[record.ID] = record
	}
	for _, record := range a {
		other, ok := byID[record.ID]
		if !ok || other.Hash != record.Hash || other.Updated != record.Updated || other.Deleted != record.Deleted {
			return false
		}
	}
	return true
}

// Status is what the settings screen shows about sync on this device.
type Status struct {
	Joined    bool
	LastSync  time.Time
	LastError string
	Records   int
}

// Status reads the local sync status without touching the storage.
func (e *Engine) Status() Status {
	status := Status{Joined: e.HasKey()}
	if value, _ := e.DB.SyncMeta(metaLastSync); value != "" {
		if unix, err := strconv.ParseInt(value, 10, 64); err == nil {
			status.LastSync = time.Unix(unix, 0)
		}
	}
	status.LastError, _ = e.DB.SyncMeta(metaLastError)
	if states, err := e.DB.LoadSyncStates(); err == nil {
		for _, state := range states {
			if !state.Deleted {
				status.Records++
			}
		}
	}
	return status
}
