package db

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"strings"
)

// SyncState is the last synchronized version of one record: its content hash,
// the time of the change that produced it, and whether it is a deletion.
type SyncState struct {
	ID      string
	Hash    string
	Updated int64
	Deleted bool
}

// ensureSyncSchema adds the columns and tables that sync needs. Server and
// forward rows get a stable sync_id because their numeric IDs are local to one
// database; groups, tags, and templates are identified by their unique names.
func (db *DB) ensureSyncSchema() error {
	for _, table := range []string{"servers", "forwards"} {
		has, err := db.hasColumn(table, "sync_id")
		if err != nil {
			return err
		}
		if !has {
			if _, err := db.conn.Exec("ALTER TABLE " + table + " ADD COLUMN sync_id TEXT NOT NULL DEFAULT ''"); err != nil {
				return fmt.Errorf("add %s.sync_id: %w", table, err)
			}
		}
	}
	if _, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS sync_records (
			record_id TEXT PRIMARY KEY,
			hash TEXT NOT NULL DEFAULT '',
			updated_at INTEGER NOT NULL DEFAULT 0,
			deleted INTEGER NOT NULL DEFAULT 0
		)`); err != nil {
		return fmt.Errorf("create sync_records: %w", err)
	}
	if _, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS sync_meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT ''
		)`); err != nil {
		return fmt.Errorf("create sync_meta: %w", err)
	}
	return nil
}

// NewSyncID returns a random RFC 4122 version 4 UUID.
func NewSyncID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("read random: %v", err))
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// EnsureSyncIDs gives every server and forward without a sync ID a new one.
func (db *DB) EnsureSyncIDs() error {
	for _, table := range []string{"servers", "forwards"} {
		rows, err := db.conn.Query("SELECT id FROM " + table + " WHERE sync_id=''")
		if err != nil {
			return err
		}
		var ids []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		rows.Close()
		for _, id := range ids {
			if _, err := db.conn.Exec("UPDATE "+table+" SET sync_id=? WHERE id=?", NewSyncID(), id); err != nil {
				return err
			}
		}
	}
	return nil
}

// ServerSyncIDs maps local server IDs to sync IDs.
func (db *DB) ServerSyncIDs() (map[int64]string, error) {
	return db.syncIDMap("servers")
}

// ForwardSyncIDs maps local forward IDs to sync IDs.
func (db *DB) ForwardSyncIDs() (map[int64]string, error) {
	return db.syncIDMap("forwards")
}

func (db *DB) syncIDMap(table string) (map[int64]string, error) {
	rows, err := db.conn.Query("SELECT id, sync_id FROM " + table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := map[int64]string{}
	for rows.Next() {
		var id int64
		var syncID string
		if err := rows.Scan(&id, &syncID); err != nil {
			return nil, err
		}
		ids[id] = syncID
	}
	return ids, rows.Err()
}

// SetServerSyncID assigns a sync ID, used when a local profile is matched to
// the same profile arriving from another device.
func (db *DB) SetServerSyncID(serverID int64, syncID string) error {
	_, err := db.conn.Exec("UPDATE servers SET sync_id=? WHERE id=?", syncID, serverID)
	return err
}

// SetForwardSyncID assigns a forward's sync ID.
func (db *DB) SetForwardSyncID(forwardID int64, syncID string) error {
	_, err := db.conn.Exec("UPDATE forwards SET sync_id=? WHERE id=?", syncID, forwardID)
	return err
}

// ServerIDBySyncID finds the local server for a sync ID.
func (db *DB) ServerIDBySyncID(syncID string) (int64, bool) {
	return db.idBySyncID("servers", syncID)
}

// ForwardIDBySyncID finds the local forward for a sync ID.
func (db *DB) ForwardIDBySyncID(syncID string) (int64, bool) {
	return db.idBySyncID("forwards", syncID)
}

func (db *DB) idBySyncID(table, syncID string) (int64, bool) {
	if syncID == "" {
		return 0, false
	}
	var id int64
	if err := db.conn.QueryRow("SELECT id FROM "+table+" WHERE sync_id=?", syncID).Scan(&id); err != nil {
		return 0, false
	}
	return id, true
}

// EnsureTag creates a tag if it does not exist.
func (db *DB) EnsureTag(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	_, err := db.conn.Exec("INSERT OR IGNORE INTO tags (name) VALUES (?)", name)
	return err
}

// EnsureGroup creates a group if it does not exist.
func (db *DB) EnsureGroup(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	_, err := db.conn.Exec("INSERT OR IGNORE INTO groups (name) VALUES (?)", name)
	return err
}

// LoadSyncStates returns the state recorded by the last successful sync.
func (db *DB) LoadSyncStates() (map[string]SyncState, error) {
	rows, err := db.conn.Query("SELECT record_id, hash, updated_at, deleted FROM sync_records")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	states := map[string]SyncState{}
	for rows.Next() {
		var state SyncState
		var deleted int
		if err := rows.Scan(&state.ID, &state.Hash, &state.Updated, &deleted); err != nil {
			return nil, err
		}
		state.Deleted = deleted != 0
		states[state.ID] = state
	}
	return states, rows.Err()
}

// ReplaceSyncStates atomically replaces the recorded sync state.
func (db *DB) ReplaceSyncStates(states []SyncState) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM sync_records"); err != nil {
		return err
	}
	stmt, err := tx.Prepare("INSERT INTO sync_records (record_id, hash, updated_at, deleted) VALUES (?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, state := range states {
		deleted := 0
		if state.Deleted {
			deleted = 1
		}
		if _, err := stmt.Exec(state.ID, state.Hash, state.Updated, deleted); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SyncMeta reads a device-local sync setting such as the last sync time.
func (db *DB) SyncMeta(key string) (string, error) {
	var value string
	err := db.conn.QueryRow("SELECT value FROM sync_meta WHERE key=?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetSyncMeta writes a device-local sync setting.
func (db *DB) SetSyncMeta(key, value string) error {
	_, err := db.conn.Exec("INSERT INTO sync_meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, value)
	return err
}

// ClearSyncState forgets every recorded sync state, used when a device leaves
// a sync space or joins a different one.
func (db *DB) ClearSyncState() error {
	if _, err := db.conn.Exec("DELETE FROM sync_records"); err != nil {
		return err
	}
	_, err := db.conn.Exec("DELETE FROM sync_meta")
	return err
}
