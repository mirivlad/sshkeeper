package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mirivlad/sshkeeper/internal/model"
)

// --- Route marshaling helpers ---

func marshalRoute(route model.Route) string {
	if len(route.Hops) == 0 {
		return ""
	}
	b, _ := json.Marshal(route.Hops)
	return string(b)
}

func unmarshalRoute(s string) model.Route {
	s = strings.TrimSpace(s)
	if s == "" {
		return model.Route{}
	}
	var hops []model.RouteHop
	if err := json.Unmarshal([]byte(s), &hops); err != nil {
		parts := strings.Split(s, ",")
		hops = make([]model.RouteHop, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				hops = append(hops, model.RouteHop{Raw: p, IsProfile: false})
			}
		}
	}
	return model.Route{Hops: hops}
}

// --- Server CRUD ---

type rowScanner interface {
	Scan(dest ...any) error
}

const serverSelectColumns = `
	id, alias, display_name, host, port, user, auth_method,
	identity_file, proxy_jump, route_hops, COALESCE(group_id, 0), group_name,
	notes, startup_command, created_at, updated_at, last_connected_at,
	last_test_at, last_test_status, last_test_error`

func scanServerBase(row rowScanner) (*model.Server, error) {
	var s model.Server
	var lastConnected, lastTest sql.NullTime
	var legacyRoute sql.NullString
	if err := row.Scan(
		&s.ID, &s.Alias, &s.DisplayName, &s.Host, &s.Port, &s.User, &s.AuthMethod,
		&s.IdentityFile, &s.ProxyJump, &legacyRoute, &s.GroupID, &s.GroupName,
		&s.Notes, &s.StartupCommand, &s.CreatedAt, &s.UpdatedAt, &lastConnected,
		&lastTest, &s.LastTestStatus, &s.LastTestError); err != nil {
		return nil, err
	}
	if lastConnected.Valid {
		s.LastConnectedAt = &lastConnected.Time
	}
	if lastTest.Valid {
		s.LastTestAt = &lastTest.Time
	}
	return &s, nil
}

func (db *DB) ResolveAlias(alias string) (int64, bool) {
	var id int64
	if err := db.conn.QueryRow(`SELECT id FROM servers WHERE alias=?`, strings.TrimSpace(alias)).Scan(&id); err != nil {
		return 0, false
	}
	return id, true
}

func (db *DB) normalizeRoute(route model.Route) (model.Route, error) {
	resolved := model.Route{Hops: make([]model.RouteHop, 0, len(route.Hops))}
	for _, hop := range route.Hops {
		if hop.Profile() {
			var id int64
			alias := strings.TrimSpace(hop.Alias)
			if hop.ServerID > 0 {
				id = hop.ServerID
				if err := db.conn.QueryRow(`SELECT alias FROM servers WHERE id=?`, id).Scan(&alias); err != nil {
					if err == sql.ErrNoRows {
						return model.Route{}, fmt.Errorf("route profile #%d not found", id)
					}
					return model.Route{}, err
				}
			} else {
				if alias == "" {
					return model.Route{}, fmt.Errorf("route profile alias is empty")
				}
				if err := db.conn.QueryRow(`SELECT id FROM servers WHERE alias=?`, alias).Scan(&id); err != nil {
					if err == sql.ErrNoRows {
						return model.Route{}, fmt.Errorf("route profile not found: %s", alias)
					}
					return model.Route{}, err
				}
			}
			resolved.Hops = append(resolved.Hops, model.RouteHop{ServerID: id, Alias: alias, IsProfile: true})
			continue
		}
		raw := strings.TrimSpace(hop.Raw)
		if raw == "" {
			return model.Route{}, fmt.Errorf("raw route hop is empty")
		}
		resolved.Hops = append(resolved.Hops, model.RouteHop{Raw: raw})
	}
	return resolved, nil
}

func (db *DB) ValidateRoute(targetID int64, route model.Route) error {
	resolved, err := db.normalizeRoute(route)
	if err != nil {
		return err
	}
	if err := model.ValidateRouteShape(targetID, resolved); err != nil {
		return err
	}
	for _, hop := range resolved.Hops {
		if !hop.Profile() {
			continue
		}
		reaches, err := db.routeReaches(hop.ServerID, targetID, map[int64]bool{})
		if err != nil {
			return err
		}
		if reaches {
			return fmt.Errorf("route cycle detected through %s", hop.Alias)
		}
	}
	return nil
}

func (db *DB) routeReaches(startID, targetID int64, visiting map[int64]bool) (bool, error) {
	if targetID > 0 && startID == targetID {
		return true, nil
	}
	if visiting[startID] {
		return false, fmt.Errorf("existing route cycle detected at server #%d", startID)
	}
	visiting[startID] = true
	defer delete(visiting, startID)
	route, err := db.loadRoute(startID)
	if err != nil {
		return false, err
	}
	for _, hop := range route.Hops {
		if !hop.Profile() {
			continue
		}
		reaches, err := db.routeReaches(hop.ServerID, targetID, visiting)
		if err != nil || reaches {
			return reaches, err
		}
	}
	return false, nil
}

func insertRouteTx(tx *sql.Tx, targetID int64, route model.Route) error {
	if _, err := tx.Exec(`DELETE FROM server_route_hops WHERE target_server_id=?`, targetID); err != nil {
		return err
	}
	for pos, hop := range route.Hops {
		if hop.Profile() {
			if _, err := tx.Exec(`INSERT INTO server_route_hops(target_server_id, position, hop_server_id) VALUES(?,?,?)`, targetID, pos, hop.ServerID); err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec(`INSERT INTO server_route_hops(target_server_id, position, raw_target) VALUES(?,?,?)`, targetID, pos, hop.Raw); err != nil {
				return err
			}
		}
	}
	return nil
}

func (db *DB) loadRoute(targetID int64) (model.Route, error) {
	rows, err := db.conn.Query(`
		SELECT h.hop_server_id, h.raw_target, COALESCE(s.alias, '')
		FROM server_route_hops h
		LEFT JOIN servers s ON s.id=h.hop_server_id
		WHERE h.target_server_id=? ORDER BY h.position`, targetID)
	if err != nil {
		return model.Route{}, err
	}
	defer rows.Close()
	route := model.Route{}
	for rows.Next() {
		var hopID sql.NullInt64
		var raw sql.NullString
		var alias string
		if err := rows.Scan(&hopID, &raw, &alias); err != nil {
			return model.Route{}, err
		}
		if hopID.Valid {
			route.Hops = append(route.Hops, model.RouteHop{ServerID: hopID.Int64, Alias: alias, IsProfile: true})
		} else if raw.Valid {
			route.Hops = append(route.Hops, model.RouteHop{Raw: raw.String})
		}
	}
	return route, rows.Err()
}

func (db *DB) routeDependents(serverID int64) ([]string, error) {
	rows, err := db.conn.Query(`
		SELECT DISTINCT s.alias FROM server_route_hops h
		JOIN servers s ON s.id=h.target_server_id
		WHERE h.hop_server_id=? ORDER BY s.alias`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var aliases []string
	for rows.Next() {
		var alias string
		if err := rows.Scan(&alias); err != nil {
			return nil, err
		}
		aliases = append(aliases, alias)
	}
	return aliases, rows.Err()
}

func (db *DB) refreshRouteCompatibility() error {
	rows, err := db.conn.Query(`SELECT id FROM servers ORDER BY id`)
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
		route, err := db.loadRoute(id)
		if err != nil {
			return err
		}
		proxy, legacy := routeCompatibilityProjection(route)
		if _, err := db.conn.Exec(`UPDATE servers SET proxy_jump=?, route_hops=? WHERE id=?`, proxy, legacy, id); err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) CreateServer(s *model.Server) error {
	resolved, err := db.normalizeRoute(s.Route)
	if err != nil {
		return err
	}
	s.Route = resolved
	s.ProxyJump = s.Route.ProxyJumpString()
	if err := model.ValidateServerBasics(s); err != nil {
		return err
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	groupID, err := ensureGroupTx(tx, s.GroupName)
	if err != nil {
		return err
	}
	proxy, legacy := routeCompatibilityProjection(s.Route)
	result, err := tx.Exec(`
		INSERT INTO servers (alias, display_name, host, port, user, auth_method, identity_file,
			proxy_jump, route_hops, group_id, group_name, notes, startup_command)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.Alias, s.DisplayName, s.Host, s.Port, s.User, s.AuthMethod, s.IdentityFile,
		proxy, legacy, nullGroupID(groupID), strings.TrimSpace(s.GroupName), s.Notes, s.StartupCommand)
	if err != nil {
		return err
	}
	s.ID, err = result.LastInsertId()
	if err != nil {
		return err
	}
	s.GroupID = groupID
	if err := model.ValidateRouteShape(s.ID, s.Route); err != nil {
		return err
	}
	if err := insertRouteTx(tx, s.ID, s.Route); err != nil {
		return err
	}
	return tx.Commit()
}

func nullGroupID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

func (db *DB) UpdateServer(s *model.Server) error {
	return db.UpdateServerByAlias(s.Alias, s)
}

func (db *DB) UpdateServerByAlias(oldAlias string, s *model.Server) error {
	var id int64
	if err := db.conn.QueryRow(`SELECT id FROM servers WHERE alias=?`, oldAlias).Scan(&id); err != nil {
		return err
	}
	resolved, err := db.normalizeRoute(s.Route)
	if err != nil {
		return err
	}
	s.ID = id
	s.Route = resolved
	s.ProxyJump = s.Route.ProxyJumpString()
	if err := model.ValidateServerBasics(s); err != nil {
		return err
	}
	if err := db.ValidateRoute(id, s.Route); err != nil {
		return err
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	groupID, err := ensureGroupTx(tx, s.GroupName)
	if err != nil {
		return err
	}
	proxy, legacy := routeCompatibilityProjection(s.Route)
	result, err := tx.Exec(`
		UPDATE servers SET alias=?, display_name=?, host=?, port=?, user=?, auth_method=?,
			identity_file=?, proxy_jump=?, route_hops=?, group_id=?, group_name=?, notes=?, startup_command=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?`,
		s.Alias, s.DisplayName, s.Host, s.Port, s.User, s.AuthMethod,
		s.IdentityFile, proxy, legacy, nullGroupID(groupID), strings.TrimSpace(s.GroupName), s.Notes, s.StartupCommand, id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return fmt.Errorf("server not found: %s", oldAlias)
	}
	if err := insertRouteTx(tx, id, s.Route); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.GroupID = groupID
	return db.refreshRouteCompatibility()
}

func (db *DB) DeleteServer(alias string) error {
	var id int64
	if err := db.conn.QueryRow(`SELECT id FROM servers WHERE alias=?`, alias).Scan(&id); err != nil {
		return err
	}
	dependents, err := db.routeDependents(id)
	if err != nil {
		return err
	}
	if len(dependents) > 0 {
		return fmt.Errorf("server %q is used as a route hop by: %s", alias, strings.Join(dependents, ", "))
	}
	_, err = db.conn.Exec(`DELETE FROM servers WHERE id=?`, id)
	return err
}

func (db *DB) loadServerByQuery(query string, arg any) (*model.Server, error) {
	s, err := scanServerBase(db.conn.QueryRow(`SELECT `+serverSelectColumns+` FROM servers WHERE `+query, arg))
	if err != nil {
		return nil, err
	}
	s.Route, err = db.loadRoute(s.ID)
	if err != nil {
		return nil, err
	}
	s.ProxyJump = s.Route.ProxyJumpString()
	s.Tags, err = db.GetServerTags(s.ID)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (db *DB) GetServer(alias string) (*model.Server, error) {
	return db.loadServerByQuery(`alias=?`, alias)
}

func (db *DB) GetServerByID(id int64) (*model.Server, error) {
	return db.loadServerByQuery(`id=?`, id)
}

func (db *DB) listServersQuery(query string, args ...any) ([]*model.Server, error) {
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var servers []*model.Server
	for rows.Next() {
		s, err := scanServerBase(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, s := range servers {
		s.Route, err = db.loadRoute(s.ID)
		if err != nil {
			return nil, err
		}
		s.ProxyJump = s.Route.ProxyJumpString()
		s.Tags, err = db.GetServerTags(s.ID)
		if err != nil {
			return nil, err
		}
	}
	return servers, nil
}

func (db *DB) ListServers() ([]*model.Server, error) {
	return db.listServersQuery(`SELECT ` + serverSelectColumns + ` FROM servers ORDER BY alias`)
}

func (db *DB) SearchServers(query string) ([]*model.Server, error) {
	pattern := "%" + query + "%"
	return db.listServersQuery(`
		SELECT `+serverSelectColumns+` FROM servers
		WHERE alias LIKE ? OR display_name LIKE ? OR host LIKE ? OR user LIKE ?
		   OR group_name LIKE ? OR notes LIKE ? OR proxy_jump LIKE ? OR route_hops LIKE ?
		   OR EXISTS (
		       SELECT 1 FROM server_route_hops rh
		       LEFT JOIN servers hs ON hs.id=rh.hop_server_id
		       WHERE rh.target_server_id=servers.id
		         AND (hs.alias LIKE ? OR rh.raw_target LIKE ?)
		   )
		   OR EXISTS (
		       SELECT 1 FROM server_tags st JOIN tags t ON t.id=st.tag_id
		       WHERE st.server_id=servers.id AND t.name LIKE ?
		   )
		   OR EXISTS (
		       SELECT 1 FROM forwards f WHERE f.server_id=servers.id
		         AND (f.name LIKE ? OR f.description LIKE ? OR f.local_addr LIKE ? OR f.remote_addr LIKE ?
		              OR CAST(f.local_port AS TEXT) LIKE ? OR CAST(f.remote_port AS TEXT) LIKE ?)
		   )
		ORDER BY alias`,
		pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern,
		pattern, pattern, pattern,
		pattern, pattern, pattern, pattern, pattern, pattern)
}

func (db *DB) UpdateTestResult(alias string, status model.TestStatus, testErr string) error {
	_, err := db.conn.Exec(`
		UPDATE servers SET last_test_at=CURRENT_TIMESTAMP, last_test_status=?, last_test_error=?
		WHERE alias=?`, status, testErr, alias)
	return err
}

func (db *DB) UpdateLastConnected(alias string) error {
	_, err := db.conn.Exec("UPDATE servers SET last_connected_at=CURRENT_TIMESTAMP WHERE alias=?", alias)
	return err
}

// --- Tag methods ---

func (db *DB) AddTagToServer(serverID int64, tagName string) error {
	tagName = strings.TrimSpace(tagName)
	if tagName == "" {
		return nil
	}
	var tagID int64
	err := db.conn.QueryRow("SELECT id FROM tags WHERE name=?", tagName).Scan(&tagID)
	if err == sql.ErrNoRows {
		result, err := db.conn.Exec("INSERT INTO tags (name) VALUES (?)", tagName)
		if err != nil {
			return err
		}
		tagID, _ = result.LastInsertId()
	} else if err != nil {
		return err
	}
	_, err = db.conn.Exec("INSERT OR IGNORE INTO server_tags (server_id, tag_id) VALUES (?, ?)", serverID, tagID)
	return err
}

func (db *DB) SetServerTags(serverID int64, tagNames []string) error {
	if _, err := db.conn.Exec("DELETE FROM server_tags WHERE server_id=?", serverID); err != nil {
		return err
	}
	for _, tagName := range uniqueCleanStrings(tagNames) {
		if err := db.AddTagToServer(serverID, tagName); err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) ListTags() ([]string, error) {
	rows, err := db.conn.Query("SELECT name FROM tags ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func (db *DB) RenameTag(oldName, newName string) error {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" || newName == "" {
		return nil
	}
	_, err := db.conn.Exec("UPDATE tags SET name=? WHERE name=?", newName, oldName)
	return err
}

func (db *DB) DeleteTag(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	_, err := db.conn.Exec("DELETE FROM tags WHERE name=?", name)
	return err
}

func (db *DB) GetServerTags(serverID int64) ([]string, error) {
	rows, err := db.conn.Query(`
		SELECT t.name FROM tags t
		JOIN server_tags st ON st.tag_id = t.id
		WHERE st.server_id = ?
		ORDER BY t.name`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tags = append(tags, name)
	}
	return tags, rows.Err()
}

// --- Forward methods ---

func (db *DB) AddForward(fwd *model.Forward) (int64, error) {
	result, err := db.conn.Exec(`
		INSERT INTO forwards (server_id, name, description, type, local_addr, local_port, remote_addr, remote_port, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fwd.ServerID, fwd.Name, fwd.Description, fwd.Type, fwd.LocalAddr, fwd.LocalPort, fwd.RemoteAddr, fwd.RemotePort, fwd.Enabled)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (db *DB) UpdateForward(fwd *model.Forward) error {
	_, err := db.conn.Exec(`
		UPDATE forwards SET name=?, description=?, type=?, local_addr=?, local_port=?, remote_addr=?, remote_port=?, enabled=?
		WHERE id=?`,
		fwd.Name, fwd.Description, fwd.Type, fwd.LocalAddr, fwd.LocalPort, fwd.RemoteAddr, fwd.RemotePort, fwd.Enabled, fwd.ID)
	return err
}

func (db *DB) GetForwards(serverID int64) ([]*model.Forward, error) {
	rows, err := db.conn.Query(`
		SELECT id, server_id, name, description, type, local_addr, local_port, remote_addr, remote_port, enabled
		FROM forwards WHERE server_id=?`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var forwards []*model.Forward
	for rows.Next() {
		var f model.Forward
		if err := rows.Scan(&f.ID, &f.ServerID, &f.Name, &f.Description, &f.Type, &f.LocalAddr, &f.LocalPort, &f.RemoteAddr, &f.RemotePort, &f.Enabled); err != nil {
			return nil, err
		}
		forwards = append(forwards, &f)
	}
	return forwards, rows.Err()
}

func (db *DB) GetForward(forwardID int64) (*model.Forward, error) {
	var f model.Forward
	err := db.conn.QueryRow(`
		SELECT id, server_id, name, description, type, local_addr, local_port, remote_addr, remote_port, enabled
		FROM forwards WHERE id=?`, forwardID).Scan(
		&f.ID, &f.ServerID, &f.Name, &f.Description, &f.Type, &f.LocalAddr, &f.LocalPort, &f.RemoteAddr, &f.RemotePort, &f.Enabled)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (db *DB) DeleteForward(forwardID int64) error {
	_, err := db.conn.Exec("DELETE FROM forwards WHERE id=?", forwardID)
	return err
}

// Ensure time import is used
var _ time.Time

// --- Command template methods ---

func (db *DB) CreateCommandTemplate(t *model.CommandTemplate) error {
	result, err := db.conn.Exec(
		"INSERT INTO global_command_templates (name, command, description) VALUES (?, ?, ?)",
		t.Name, t.Command, t.Description)
	if err != nil {
		return err
	}
	t.ID, _ = result.LastInsertId()
	return err
}

func (db *DB) GetCommandTemplate(name string) (*model.CommandTemplate, error) {
	var t model.CommandTemplate
	err := db.conn.QueryRow(`
		SELECT id, name, command, description
		FROM global_command_templates WHERE name=?`, name).Scan(&t.ID, &t.Name, &t.Command, &t.Description)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (db *DB) ListCommandTemplates() ([]*model.CommandTemplate, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, command, description
		FROM global_command_templates
		ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []*model.CommandTemplate
	for rows.Next() {
		var t model.CommandTemplate
		if err := rows.Scan(&t.ID, &t.Name, &t.Command, &t.Description); err != nil {
			return nil, err
		}
		templates = append(templates, &t)
	}
	return templates, rows.Err()
}

func (db *DB) UpdateCommandTemplate(oldName string, t *model.CommandTemplate) error {
	_, err := db.conn.Exec(`
		UPDATE global_command_templates
		SET name=?, command=?, description=?, updated_at=CURRENT_TIMESTAMP
		WHERE name=?`, t.Name, t.Command, t.Description, oldName)
	return err
}

func (db *DB) DeleteCommandTemplate(name string) error {
	_, err := db.conn.Exec("DELETE FROM global_command_templates WHERE name=?", name)
	return err
}

func uniqueCleanStrings(values []string) []string {
	seen := map[string]bool{}
	var result []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

// --- Group methods ---

func (db *DB) CreateGroup(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("group name is required")
	}
	_, err := db.conn.Exec(`INSERT INTO groups(name) VALUES(?)`, name)
	return err
}

func (db *DB) ListGroups() ([]*model.Group, error) {
	rows, err := db.conn.Query(`
		SELECT g.id, g.name, count(s.id)
		FROM groups g LEFT JOIN servers s ON s.group_id=g.id
		GROUP BY g.id, g.name ORDER BY g.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var groups []*model.Group
	for rows.Next() {
		var group model.Group
		if err := rows.Scan(&group.ID, &group.Name, &group.ServerCount); err != nil {
			return nil, err
		}
		groups = append(groups, &group)
	}
	return groups, rows.Err()
}

func (db *DB) GetGroups() ([]string, error) {
	groups, err := db.ListGroups()
	if err != nil {
		return nil, err
	}
	names := make([]string, len(groups))
	for i, group := range groups {
		names[i] = group.Name
	}
	return names, nil
}

func (db *DB) RenameGroup(oldName, newName string) error {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" || newName == "" {
		return fmt.Errorf("group name is required")
	}
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id int64
	if err := tx.QueryRow(`SELECT id FROM groups WHERE name=?`, oldName).Scan(&id); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE groups SET name=? WHERE id=?`, newName, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE servers SET group_name=?, updated_at=CURRENT_TIMESTAMP WHERE group_id=?`, newName, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) DeleteGroup(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id int64
	if err := tx.QueryRow(`SELECT id FROM groups WHERE name=?`, name).Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}
	if _, err := tx.Exec(`UPDATE servers SET group_id=NULL, group_name='', updated_at=CURRENT_TIMESTAMP WHERE group_id=?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM groups WHERE id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}
