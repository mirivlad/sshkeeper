package db

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/mirivlad/sshkeeper/internal/model"
)

func (db *DB) ensureV040Schema() error {
	if _, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS groups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE
		)`); err != nil {
		return fmt.Errorf("create groups: %w", err)
	}

	hasGroupID, err := db.hasColumn("servers", "group_id")
	if err != nil {
		return err
	}
	if !hasGroupID {
		if _, err := db.conn.Exec("ALTER TABLE servers ADD COLUMN group_id INTEGER"); err != nil {
			return fmt.Errorf("add servers.group_id: %w", err)
		}
	}

	if _, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS server_route_hops (
			target_server_id INTEGER NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
			position INTEGER NOT NULL,
			hop_server_id INTEGER REFERENCES servers(id) ON DELETE RESTRICT,
			raw_target TEXT,
			PRIMARY KEY (target_server_id, position),
			CHECK ((hop_server_id IS NOT NULL AND raw_target IS NULL) OR
			       (hop_server_id IS NULL AND raw_target IS NOT NULL AND length(trim(raw_target)) > 0))
		)`); err != nil {
		return fmt.Errorf("create server_route_hops: %w", err)
	}
	if _, err := db.conn.Exec(`CREATE INDEX IF NOT EXISTS idx_route_hop_profile ON server_route_hops(hop_server_id)`); err != nil {
		return fmt.Errorf("index server_route_hops: %w", err)
	}

	if err := db.migrateLegacyGroups(); err != nil {
		return err
	}
	if err := db.migrateLegacyRoutes(); err != nil {
		return err
	}
	return nil
}

func (db *DB) migrateLegacyGroups() error {
	if _, err := db.conn.Exec(`
		INSERT OR IGNORE INTO groups(name)
		SELECT DISTINCT trim(group_name) FROM servers WHERE trim(group_name) != ''`); err != nil {
		return fmt.Errorf("migrate groups: %w", err)
	}
	if _, err := db.conn.Exec(`
		UPDATE servers
		SET group_id = (SELECT id FROM groups WHERE groups.name = servers.group_name)
		WHERE group_id IS NULL AND trim(group_name) != ''`); err != nil {
		return fmt.Errorf("link migrated groups: %w", err)
	}
	return nil
}

func (db *DB) migrateLegacyRoutes() error {
	rows, err := db.conn.Query(`SELECT id, proxy_jump, route_hops FROM servers ORDER BY id`)
	if err != nil {
		return fmt.Errorf("read legacy routes: %w", err)
	}
	defer rows.Close()

	type legacyServer struct {
		id        int64
		proxyJump string
		routeHops string
	}
	var legacy []legacyServer
	for rows.Next() {
		var item legacyServer
		if err := rows.Scan(&item.id, &item.proxyJump, &item.routeHops); err != nil {
			return err
		}
		legacy = append(legacy, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, item := range legacy {
		var count int
		if err := db.conn.QueryRow(`SELECT count(*) FROM server_route_hops WHERE target_server_id=?`, item.id).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		source := strings.TrimSpace(item.routeHops)
		if source == "" {
			source = strings.TrimSpace(item.proxyJump)
		}
		if source == "" {
			continue
		}
		route := unmarshalRoute(source)
		for pos, hop := range route.Hops {
			hopID := hop.ServerID
			candidate := strings.TrimSpace(hop.Alias)
			if candidate == "" {
				candidate = strings.TrimSpace(hop.Raw)
			}
			if hopID == 0 && candidate != "" {
				_ = db.conn.QueryRow(`SELECT id FROM servers WHERE alias=?`, candidate).Scan(&hopID)
			}
			if hopID > 0 && hopID != item.id {
				if _, err := db.conn.Exec(`INSERT INTO server_route_hops(target_server_id, position, hop_server_id) VALUES(?,?,?)`, item.id, pos, hopID); err != nil {
					return fmt.Errorf("migrate route hop: %w", err)
				}
				continue
			}
			raw := candidate
			if raw == "" {
				raw = strings.TrimSpace(hop.Raw)
			}
			if raw == "" {
				continue
			}
			if _, err := db.conn.Exec(`INSERT INTO server_route_hops(target_server_id, position, raw_target) VALUES(?,?,?)`, item.id, pos, raw); err != nil {
				return fmt.Errorf("migrate raw route hop: %w", err)
			}
		}
	}
	return nil
}

func ensureGroupTx(tx *sql.Tx, name string) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, nil
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO groups(name) VALUES(?)`, name); err != nil {
		return 0, err
	}
	var id int64
	if err := tx.QueryRow(`SELECT id FROM groups WHERE name=?`, name).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func routeCompatibilityProjection(route model.Route) (proxyJump, routeJSON string) {
	proxyJump = route.ProxyJumpString()
	routeJSON = marshalRoute(route)
	return proxyJump, routeJSON
}
