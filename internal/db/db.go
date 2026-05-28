package db

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type DB struct {
	conn *sql.DB
}

func Open(dataDir string) (*DB, error) {
	dbPath := filepath.Join(dataDir, "sshkeeper.db")

	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	db := &DB{conn: conn}

	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	if err := db.ensureSchema(); err != nil {
		return nil, fmt.Errorf("ensure schema: %w", err)
	}

	os.Chmod(dbPath, 0600)

	return db, nil
}

func (db *DB) ensureSchema() error {
	hasStartupCommand, err := db.hasColumn("servers", "startup_command")
	if err != nil {
		return err
	}
	if !hasStartupCommand {
		if _, err := db.conn.Exec("ALTER TABLE servers ADD COLUMN startup_command TEXT NOT NULL DEFAULT ''"); err != nil {
			return fmt.Errorf("add startup_command: %w", err)
		}
	}

	_, err = db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS global_command_templates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			command TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`)
	if err != nil {
		return fmt.Errorf("create global templates: %w", err)
	}

	_, err = db.conn.Exec(`
		INSERT OR IGNORE INTO global_command_templates (name, command)
		SELECT name, command FROM command_templates`)
	if err != nil {
		return fmt.Errorf("copy legacy templates: %w", err)
	}
	return nil
}

func (db *DB) hasColumn(tableName, columnName string) (bool, error) {
	rows, err := db.conn.Query("PRAGMA table_info(" + tableName + ")")
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return false, err
		}
		if name == columnName {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) migrate() error {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		content, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		if _, err := db.conn.Exec(string(content)); err != nil {
			return fmt.Errorf("exec migration %s: %w", entry.Name(), err)
		}
	}

	return nil
}
