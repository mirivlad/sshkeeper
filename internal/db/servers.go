package db

import (
	"database/sql"
	"sort"
	"strings"
	"time"

	"github.com/mirivlad/sshkeeper/internal/model"
)

func (db *DB) CreateServer(s *model.Server) error {
	result, err := db.conn.Exec(`
		INSERT INTO servers (alias, display_name, host, port, user, auth_method, identity_file, proxy_jump, group_name, notes, startup_command)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.Alias, s.DisplayName, s.Host, s.Port, s.User, s.AuthMethod, s.IdentityFile, s.ProxyJump, s.GroupName, s.Notes, s.StartupCommand)
	if err != nil {
		return err
	}
	s.ID, _ = result.LastInsertId()
	return nil
}

func (db *DB) UpdateServer(s *model.Server) error {
	_, err := db.conn.Exec(`
		UPDATE servers SET
			display_name=?, host=?, port=?, user=?, auth_method=?,
			identity_file=?, proxy_jump=?, group_name=?, notes=?, startup_command=?, updated_at=CURRENT_TIMESTAMP
		WHERE alias=?`,
		s.DisplayName, s.Host, s.Port, s.User, s.AuthMethod,
		s.IdentityFile, s.ProxyJump, s.GroupName, s.Notes, s.StartupCommand, s.Alias)
	return err
}

func (db *DB) UpdateServerByAlias(oldAlias string, s *model.Server) error {
	_, err := db.conn.Exec(`
		UPDATE servers SET
			alias=?, display_name=?, host=?, port=?, user=?, auth_method=?,
			identity_file=?, proxy_jump=?, group_name=?, notes=?, startup_command=?, updated_at=CURRENT_TIMESTAMP
		WHERE alias=?`,
		s.Alias, s.DisplayName, s.Host, s.Port, s.User, s.AuthMethod,
		s.IdentityFile, s.ProxyJump, s.GroupName, s.Notes, s.StartupCommand, oldAlias)
	return err
}

func (db *DB) DeleteServer(alias string) error {
	_, err := db.conn.Exec("DELETE FROM servers WHERE alias=?", alias)
	return err
}

func (db *DB) GetServer(alias string) (*model.Server, error) {
	var s model.Server
	var lastConnected, lastTest sql.NullTime
	err := db.conn.QueryRow(`
		SELECT id, alias, display_name, host, port, user, auth_method,
		       identity_file, proxy_jump, group_name, notes, startup_command,
		       created_at, updated_at, last_connected_at,
		       last_test_at, last_test_status, last_test_error
		FROM servers WHERE alias=?`, alias).Scan(
		&s.ID, &s.Alias, &s.DisplayName, &s.Host, &s.Port, &s.User, &s.AuthMethod,
		&s.IdentityFile, &s.ProxyJump, &s.GroupName, &s.Notes, &s.StartupCommand,
		&s.CreatedAt, &s.UpdatedAt, &lastConnected,
		&lastTest, &s.LastTestStatus, &s.LastTestError)
	if err != nil {
		return nil, err
	}
	if lastConnected.Valid {
		s.LastConnectedAt = &lastConnected.Time
	}
	if lastTest.Valid {
		s.LastTestAt = &lastTest.Time
	}
	tags, err := db.GetServerTags(s.ID)
	if err != nil {
		return nil, err
	}
	s.Tags = tags
	return &s, nil
}

func (db *DB) ListServers() ([]*model.Server, error) {
	rows, err := db.conn.Query(`
		SELECT id, alias, display_name, host, port, user, auth_method,
		       identity_file, proxy_jump, group_name, notes, startup_command,
		       created_at, updated_at, last_connected_at,
		       last_test_at, last_test_status, last_test_error
		FROM servers ORDER BY alias`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []*model.Server
	for rows.Next() {
		var s model.Server
		var lastConnected, lastTest sql.NullTime
		err := rows.Scan(
			&s.ID, &s.Alias, &s.DisplayName, &s.Host, &s.Port, &s.User, &s.AuthMethod,
			&s.IdentityFile, &s.ProxyJump, &s.GroupName, &s.Notes, &s.StartupCommand,
			&s.CreatedAt, &s.UpdatedAt, &lastConnected,
			&lastTest, &s.LastTestStatus, &s.LastTestError)
		if err != nil {
			return nil, err
		}
		if lastConnected.Valid {
			s.LastConnectedAt = &lastConnected.Time
		}
		if lastTest.Valid {
			s.LastTestAt = &lastTest.Time
		}
		tags, err := db.GetServerTags(s.ID)
		if err != nil {
			return nil, err
		}
		s.Tags = tags
		servers = append(servers, &s)
	}
	return servers, rows.Err()
}

func (db *DB) SearchServers(query string) ([]*model.Server, error) {
	pattern := "%" + query + "%"
	rows, err := db.conn.Query(`
		SELECT id, alias, display_name, host, port, user, auth_method,
		       identity_file, proxy_jump, group_name, notes, startup_command,
		       created_at, updated_at, last_connected_at,
		       last_test_at, last_test_status, last_test_error
		FROM servers
		WHERE alias LIKE ? OR display_name LIKE ? OR host LIKE ? OR user LIKE ? OR group_name LIKE ?
		ORDER BY alias`, pattern, pattern, pattern, pattern, pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []*model.Server
	for rows.Next() {
		var s model.Server
		var lastConnected, lastTest sql.NullTime
		err := rows.Scan(
			&s.ID, &s.Alias, &s.DisplayName, &s.Host, &s.Port, &s.User, &s.AuthMethod,
			&s.IdentityFile, &s.ProxyJump, &s.GroupName, &s.Notes, &s.StartupCommand,
			&s.CreatedAt, &s.UpdatedAt, &lastConnected,
			&lastTest, &s.LastTestStatus, &s.LastTestError)
		if err != nil {
			return nil, err
		}
		if lastConnected.Valid {
			s.LastConnectedAt = &lastConnected.Time
		}
		if lastTest.Valid {
			s.LastTestAt = &lastTest.Time
		}
		tags, err := db.GetServerTags(s.ID)
		if err != nil {
			return nil, err
		}
		s.Tags = tags
		servers = append(servers, &s)
	}
	return servers, rows.Err()
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

// Tag methods
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

// Forward methods
func (db *DB) AddForward(serverID int64, fwdType model.ForwardType, localAddr string, localPort int, remoteAddr string, remotePort int) error {
	_, err := db.conn.Exec(`
		INSERT INTO forwards (server_id, type, local_addr, local_port, remote_addr, remote_port)
		VALUES (?, ?, ?, ?, ?, ?)`,
		serverID, fwdType, localAddr, localPort, remoteAddr, remotePort)
	return err
}

func (db *DB) GetForwards(serverID int64) ([]*model.Forward, error) {
	rows, err := db.conn.Query(`
		SELECT id, server_id, type, local_addr, local_port, remote_addr, remote_port
		FROM forwards WHERE server_id=?`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var forwards []*model.Forward
	for rows.Next() {
		var f model.Forward
		if err := rows.Scan(&f.ID, &f.ServerID, &f.Type, &f.LocalAddr, &f.LocalPort, &f.RemoteAddr, &f.RemotePort); err != nil {
			return nil, err
		}
		forwards = append(forwards, &f)
	}
	return forwards, rows.Err()
}

// Ensure time import is used
var _ time.Time

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

// GetGroups returns all unique group names with server count
func (db *DB) GetGroups() ([]string, error) {
	rows, err := db.conn.Query(`
		SELECT group_name FROM servers 
		WHERE group_name != '' 
		GROUP BY group_name 
		ORDER BY group_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		groups = append(groups, name)
	}
	return groups, rows.Err()
}

// RenameGroup renames a group for all servers in it
func (db *DB) RenameGroup(oldName, newName string) error {
	_, err := db.conn.Exec(
		"UPDATE servers SET group_name = ?, updated_at = CURRENT_TIMESTAMP WHERE group_name = ?",
		newName, oldName)
	return err
}

// DeleteGroup removes group assignment from all servers
func (db *DB) DeleteGroup(name string) error {
	_, err := db.conn.Exec(
		"UPDATE servers SET group_name = '', updated_at = CURRENT_TIMESTAMP WHERE group_name = ?",
		name)
	return err
}
