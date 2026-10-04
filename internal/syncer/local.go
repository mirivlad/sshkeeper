package syncer

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	slashpath "path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mirivlad/sshkeeper/internal/db"
	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/mirivlad/sshkeeper/internal/vault"
)

// secretTypes are the vault secrets kept per profile.
var secretTypes = []string{"ssh_password", "key_passphrase", "sudo_password"}

// maxKeySize bounds what is read as a private key file.
const maxKeySize = 64 << 10

// Local reads and writes this device's data: the database, the vault, and
// private key files.
type Local struct {
	DB    *db.DB
	Vault *vault.Vault
	// Home expands "~" in identity file paths.
	Home string

	// unreadable holds records that exist but could not be read during the
	// last Export. They must not be mistaken for deletions.
	unreadable map[string]bool
}

func stableSecretID(serverID int64, secretType string) string {
	return fmt.Sprintf("server-id:%d:%s", serverID, secretType)
}

func legacySecretID(alias, secretType string) string {
	return fmt.Sprintf("server:%s:%s", alias, secretType)
}

// expand resolves "~/" against Home. Relative paths stay relative to Home
// too, matching how ssh resolves them from the user's shell.
func (l *Local) expand(path string) string {
	switch {
	case path == "~":
		return l.Home
	case strings.HasPrefix(path, "~/"):
		return filepath.Join(l.Home, path[2:])
	case filepath.IsAbs(path):
		return path
	default:
		return filepath.Join(l.Home, path)
	}
}

// portableKeyPath converts a device-local identity path into a path that is
// meaningful on every OS. Keys under .ssh keep their relative name; arbitrary
// external paths are copied into a deterministic sshkeeper-managed location.
//
// The text checks intentionally understand both slash styles so a v0.7.0/0.7.1
// bundle made on Linux can be adopted on Windows and vice versa.
func (l *Local) portableKeyPath(source string) string {
	raw := strings.TrimSpace(source)
	slash := strings.ReplaceAll(raw, "\\", "/")
	if portable, ok := portableHomePath(slash); ok {
		return portable
	}

	// A foreign absolute path may still clearly identify a key below .ssh.
	lower := strings.ToLower(slash)
	if index := strings.LastIndex(lower, "/.ssh/"); index >= 0 {
		relative := slash[index+len("/.ssh/"):]
		if clean, ok := safeSlashRelative(relative); ok {
			return "~/.ssh/" + clean
		}
	}

	// For a path native to this OS, preserve anything below the user's home.
	full := l.expand(raw)
	homeAbs, homeErr := filepath.Abs(l.Home)
	fullAbs, fullErr := filepath.Abs(full)
	if homeErr == nil && fullErr == nil {
		if relative, err := filepath.Rel(homeAbs, fullAbs); err == nil && relative != "." &&
			relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return "~/" + filepath.ToSlash(relative)
		}
	}

	// An arbitrary external path has no portable equivalent. Give it a stable,
	// collision-resistant managed name without exposing the original directory.
	base := slashpath.Base(slash)
	base = sanitizeKeyName(base)
	sum := sha256.Sum256([]byte(slash))
	return fmt.Sprintf("~/.ssh/sshkeeper/%s-%x", base, sum[:8])
}

func portableHomePath(slash string) (string, bool) {
	if slash == "~" {
		return "", false
	}
	if !strings.HasPrefix(slash, "~/") {
		return "", false
	}
	relative, ok := safeSlashRelative(strings.TrimPrefix(slash, "~/"))
	if !ok {
		return "", false
	}
	return "~/" + relative, true
}

func safeSlashRelative(value string) (string, bool) {
	clean := slashpath.Clean(strings.TrimSpace(value))
	if clean == "" || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || slashpath.IsAbs(clean) {
		return "", false
	}
	return clean, true
}

func sanitizeKeyName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "." || value == "/" {
		return "key"
	}
	var out strings.Builder
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z',
			char >= 'A' && char <= 'Z',
			char >= '0' && char <= '9',
			char == '.', char == '_', char == '-':
			out.WriteRune(char)
		default:
			out.WriteByte('_')
		}
	}
	if out.Len() == 0 {
		return "key"
	}
	return out.String()
}

func (l *Local) localIdentityPath(portable string) string {
	// Keep the portable ~/ prefix intact until expand sees it. filepath.Join
	// will translate the remaining separators for the local OS.
	return l.expand(portable)
}

// Export returns every local item as a record without a timestamp. Warnings
// name items that could not be read (for example an unreadable key file).
func (l *Local) Export() ([]Record, []string, error) {
	if err := l.DB.EnsureSyncIDs(); err != nil {
		return nil, nil, err
	}
	servers, err := l.DB.ListServers()
	if err != nil {
		return nil, nil, err
	}
	serverIDs, err := l.DB.ServerSyncIDs()
	if err != nil {
		return nil, nil, err
	}
	var records []Record
	var warnings []string
	add := func(kind, key string, data any) error {
		record, err := NewRecord(kind, key, data)
		if err == nil {
			records = append(records, record)
		}
		return err
	}

	l.unreadable = map[string]bool{}

	// Identity paths are device-local. Export keys under portable home-relative
	// names so the same encrypted bundle can move between Linux, macOS, and
	// Windows without carrying /home/... or C:\Users\... into another OS.
	keyPaths := map[string]string{} // local source path -> portable sync path
	for _, server := range servers {
		if source := strings.TrimSpace(server.IdentityFile); source != "" {
			keyPaths[source] = l.portableKeyPath(source)
		}
	}
	emittedKeys := map[string]bool{}
	for source, portable := range keyPaths {
		data, err := l.readKey(source)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				warnings = append(warnings, fmt.Sprintf("key %s: %v", source, err))
			}
			// Preserve an existing synced key instead of turning a temporary local
			// read failure into a deletion. During migration from v0.7.0/v0.7.1
			// the state may still use the old absolute-path record ID, so protect
			// both the new portable ID and the legacy source ID.
			l.unreadable[KindKey+":"+portable] = true
			if source != portable {
				l.unreadable[KindKey+":"+source] = true
			}
			continue
		}
		if emittedKeys[portable] {
			continue
		}
		data.Path = portable
		if err := add(KindKey, portable, data); err != nil {
			return nil, nil, err
		}
		emittedKeys[portable] = true
	}

	for _, server := range servers {
		uuid := serverIDs[server.ID]
		identity := server.IdentityFile
		if portable, ok := keyPaths[strings.TrimSpace(identity)]; ok {
			identity = portable
		}
		data := ServerData{
			Alias: server.Alias, DisplayName: server.DisplayName, Host: server.Host, Port: server.Port,
			User: server.User, AuthMethod: string(server.AuthMethod), IdentityFile: identity,
			Group: server.GroupName, Notes: server.Notes, StartupCommand: server.StartupCommand,
			Tags: append([]string(nil), server.Tags...),
		}
		sort.Strings(data.Tags)
		for _, hop := range server.Route.Hops {
			if hop.Profile() {
				data.Route = append(data.Route, HopData{Server: serverIDs[hop.ServerID], Alias: hop.Alias})
			} else {
				data.Route = append(data.Route, HopData{Raw: hop.Raw})
			}
		}
		if err := add(KindServer, uuid, data); err != nil {
			return nil, nil, err
		}
		for _, secretType := range secretTypes {
			value, err := l.Vault.Get(stableSecretID(server.ID, secretType))
			if err != nil {
				value, err = l.Vault.Get(legacySecretID(server.Alias, secretType))
			}
			if err != nil {
				continue
			}
			if err := add(KindSecret, uuid+":"+secretType, SecretData{Server: uuid, Type: secretType, Value: value}); err != nil {
				return nil, nil, err
			}
		}
	}

	forwards, err := l.DB.ListAllForwards()
	if err != nil {
		return nil, nil, err
	}
	forwardIDs, err := l.DB.ForwardSyncIDs()
	if err != nil {
		return nil, nil, err
	}
	for _, fwd := range forwards {
		data := ForwardData{
			Server: serverIDs[fwd.ServerID], Name: fwd.Name, Description: fwd.Description, Type: string(fwd.Type),
			LocalAddr: fwd.LocalAddr, LocalPort: fwd.LocalPort, RemoteAddr: fwd.RemoteAddr, RemotePort: fwd.RemotePort,
			Enabled: fwd.Enabled,
		}
		if err := add(KindForward, forwardIDs[fwd.ID], data); err != nil {
			return nil, nil, err
		}
	}

	groups, err := l.DB.GetGroups()
	if err != nil {
		return nil, nil, err
	}
	for _, name := range groups {
		if err := add(KindGroup, name, NameData{Name: name}); err != nil {
			return nil, nil, err
		}
	}
	tags, err := l.DB.ListTags()
	if err != nil {
		return nil, nil, err
	}
	for _, name := range tags {
		if err := add(KindTag, name, NameData{Name: name}); err != nil {
			return nil, nil, err
		}
	}
	templates, err := l.DB.ListCommandTemplates()
	if err != nil {
		return nil, nil, err
	}
	for _, template := range templates {
		if err := add(KindTemplate, template.Name, TemplateData{Name: template.Name, Command: template.Command, Description: template.Description}); err != nil {
			return nil, nil, err
		}
	}
	SortRecords(records)
	return records, warnings, nil
}

func (l *Local) readKey(path string) (KeyData, error) {
	full := l.expand(path)
	info, err := os.Stat(full)
	if err != nil {
		return KeyData{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxKeySize {
		return KeyData{}, fmt.Errorf("not a key file")
	}
	private, err := os.ReadFile(full)
	if err != nil {
		return KeyData{}, err
	}
	data := KeyData{Path: path, Private: private}
	if public, err := os.ReadFile(full + ".pub"); err == nil && len(public) <= maxKeySize {
		data.Public = public
	}
	return data, nil
}

// Adopt links local items that were never synced to identical items arriving
// from another device, so joining a space does not duplicate profiles that
// exist on both devices. A server is matched by alias, a forward by its
// server and ports.
func (l *Local) Adopt(remote []Record, states map[string]Record) error {
	if err := l.DB.EnsureSyncIDs(); err != nil {
		return err
	}
	serverIDs, err := l.DB.ServerSyncIDs()
	if err != nil {
		return err
	}
	remoteIDs := map[string]bool{}
	for _, record := range remote {
		remoteIDs[record.ID] = true
	}
	unsynced := func(kind, uuid string) bool {
		id := kind + ":" + uuid
		return !remoteIDs[id] && states[id].ID == ""
	}
	for _, record := range remote {
		if record.Deleted || record.Kind != KindServer {
			continue
		}
		uuid := strings.TrimPrefix(record.ID, KindServer+":")
		if _, ok := l.DB.ServerIDBySyncID(uuid); ok {
			continue
		}
		var data ServerData
		if json.Unmarshal(record.Data, &data) != nil {
			continue
		}
		localID, ok := l.DB.ResolveAlias(data.Alias)
		if !ok || !unsynced(KindServer, serverIDs[localID]) {
			continue
		}
		if err := l.DB.SetServerSyncID(localID, uuid); err != nil {
			return err
		}
		serverIDs[localID] = uuid
	}

	forwards, err := l.DB.ListAllForwards()
	if err != nil {
		return err
	}
	forwardIDs, err := l.DB.ForwardSyncIDs()
	if err != nil {
		return err
	}
	for _, record := range remote {
		if record.Deleted || record.Kind != KindForward {
			continue
		}
		uuid := strings.TrimPrefix(record.ID, KindForward+":")
		if _, ok := l.DB.ForwardIDBySyncID(uuid); ok {
			continue
		}
		var data ForwardData
		if json.Unmarshal(record.Data, &data) != nil {
			continue
		}
		for _, fwd := range forwards {
			if serverIDs[fwd.ServerID] == data.Server && string(fwd.Type) == data.Type &&
				fwd.LocalPort == data.LocalPort && fwd.RemoteAddr == data.RemoteAddr && fwd.RemotePort == data.RemotePort &&
				unsynced(KindForward, forwardIDs[fwd.ID]) {
				if err := l.DB.SetForwardSyncID(fwd.ID, uuid); err != nil {
					return err
				}
				forwardIDs[fwd.ID] = uuid
				break
			}
		}
	}
	return nil
}

// Apply writes incoming records to this device. It returns the number of
// items changed and warnings for items it could not apply; one failed item
// does not stop the others.
func (l *Local) Apply(incoming []Record) (int, []string, error) {
	byKind := map[string][]Record{}
	for _, record := range incoming {
		byKind[record.Kind] = append(byKind[record.Kind], record)
	}
	applied := 0
	var warnings []string
	warn := func(record Record, err error) {
		warnings = append(warnings, fmt.Sprintf("%s: %v", describe(record), err))
	}
	upserts := func(kind string) []Record {
		var list []Record
		for _, record := range byKind[kind] {
			if !record.Deleted {
				list = append(list, record)
			}
		}
		return list
	}
	deletions := func(kind string) []Record {
		var list []Record
		for _, record := range byKind[kind] {
			if record.Deleted {
				list = append(list, record)
			}
		}
		return list
	}

	// Decode key records before profiles. Their source path may come from a
	// different operating system; profiles must point to the local destination
	// rather than retaining that foreign path.
	keyData := map[string]KeyData{}
	keyDestinations := map[string]string{} // path stored in bundle -> portable path
	for _, record := range upserts(KindKey) {
		var data KeyData
		if err := decode(record, &data); err != nil {
			warn(record, err)
			continue
		}
		source := strings.TrimSpace(data.Path)
		portable := l.portableKeyPath(source)
		keyDestinations[source] = portable
		data.Path = portable
		keyData[record.ID] = data
	}

	for _, record := range upserts(KindGroup) {
		var data NameData
		if err := decode(record, &data); err != nil || l.DB.EnsureGroup(data.Name) != nil {
			warn(record, fmt.Errorf("cannot create group"))
			continue
		}
		applied++
	}
	for _, record := range upserts(KindTag) {
		var data NameData
		if err := decode(record, &data); err != nil || l.DB.EnsureTag(data.Name) != nil {
			warn(record, fmt.Errorf("cannot create tag"))
			continue
		}
		applied++
	}
	for _, record := range upserts(KindTemplate) {
		var data TemplateData
		if err := decode(record, &data); err != nil {
			warn(record, err)
			continue
		}
		template := &model.CommandTemplate{Name: data.Name, Command: data.Command, Description: data.Description}
		var err error
		if _, getErr := l.DB.GetCommandTemplate(data.Name); getErr == nil {
			err = l.DB.UpdateCommandTemplate(data.Name, template)
		} else {
			err = l.DB.CreateCommandTemplate(template)
		}
		if err != nil {
			warn(record, err)
			continue
		}
		applied++
	}

	// Servers go in two passes: create missing profiles without routes, then
	// write every profile with routes, so a route can point at a profile that
	// arrives in the same sync.
	servers := upserts(KindServer)
	decoded := make([]ServerData, len(servers))
	for index, record := range servers {
		if err := decode(record, &decoded[index]); err != nil {
			warn(record, err)
			continue
		}
		identity := strings.TrimSpace(decoded[index].IdentityFile)
		if portable, ok := keyDestinations[identity]; ok {
			decoded[index].IdentityFile = l.localIdentityPath(portable)
		} else if portable, ok := portableHomePath(strings.ReplaceAll(identity, "\\", "/")); ok {
			// New-format bundles keep portable paths even if the key record itself
			// did not change in this merge.
			decoded[index].IdentityFile = l.localIdentityPath(portable)
		}
		uuid := strings.TrimPrefix(record.ID, KindServer+":")
		if _, ok := l.DB.ServerIDBySyncID(uuid); ok {
			continue
		}
		data := decoded[index]
		if _, taken := l.DB.ResolveAlias(data.Alias); taken {
			// A different profile already uses this alias on this device.
			data.Alias = data.Alias + "-sync"
			decoded[index].Alias = data.Alias
			warnings = append(warnings, fmt.Sprintf("profile %q exists on this device; the synced one is saved as %q", strings.TrimSuffix(data.Alias, "-sync"), data.Alias))
		}
		server := l.serverModel(data, false)
		if err := l.DB.CreateServer(server); err != nil {
			warn(record, err)
			continue
		}
		if err := l.DB.SetServerSyncID(server.ID, uuid); err != nil {
			return applied, warnings, err
		}
	}
	for index, record := range servers {
		if decoded[index].Alias == "" {
			continue
		}
		uuid := strings.TrimPrefix(record.ID, KindServer+":")
		localID, ok := l.DB.ServerIDBySyncID(uuid)
		if !ok {
			continue
		}
		current, err := l.DB.GetServerByID(localID)
		if err != nil {
			warn(record, err)
			continue
		}
		server := l.serverModel(decoded[index], true)
		if err := l.DB.UpdateServerByAlias(current.Alias, server); err != nil {
			warn(record, err)
			continue
		}
		if err := l.DB.SetServerTags(localID, decoded[index].Tags); err != nil {
			warn(record, err)
			continue
		}
		applied++
	}

	for _, record := range upserts(KindForward) {
		var data ForwardData
		if err := decode(record, &data); err != nil {
			warn(record, err)
			continue
		}
		serverID, ok := l.DB.ServerIDBySyncID(data.Server)
		if !ok {
			warn(record, fmt.Errorf("its profile is missing"))
			continue
		}
		fwd := &model.Forward{
			ServerID: serverID, Name: data.Name, Description: data.Description, Type: model.ForwardType(data.Type),
			LocalAddr: data.LocalAddr, LocalPort: data.LocalPort, RemoteAddr: data.RemoteAddr, RemotePort: data.RemotePort,
			Enabled: data.Enabled,
		}
		uuid := strings.TrimPrefix(record.ID, KindForward+":")
		var err error
		if id, exists := l.DB.ForwardIDBySyncID(uuid); exists {
			fwd.ID = id
			err = l.DB.UpdateForward(fwd)
		} else {
			var id int64
			if id, err = l.DB.AddForward(fwd); err == nil {
				err = l.DB.SetForwardSyncID(id, uuid)
			}
		}
		if err != nil {
			warn(record, err)
			continue
		}
		applied++
	}

	vaultChanged := false
	for _, record := range upserts(KindSecret) {
		var data SecretData
		if err := decode(record, &data); err != nil {
			warn(record, err)
			continue
		}
		serverID, ok := l.DB.ServerIDBySyncID(data.Server)
		if !ok {
			warn(record, fmt.Errorf("its profile is missing"))
			continue
		}
		if err := l.Vault.Put(stableSecretID(serverID, data.Type), data.Type, data.Value); err != nil {
			warn(record, err)
			continue
		}
		vaultChanged = true
		applied++
	}
	for _, record := range upserts(KindKey) {
		data, ok := keyData[record.ID]
		if !ok {
			continue
		}
		written, err := l.writeKey(data)
		if err != nil {
			warn(record, err)
			continue
		}
		if written {
			applied++
		}
	}

	// Deletions run after upserts: a profile that stops being a route hop is
	// updated before it is removed.
	for _, record := range deletions(KindSecret) {
		parts := strings.SplitN(strings.TrimPrefix(record.ID, KindSecret+":"), ":", 2)
		if len(parts) != 2 {
			continue
		}
		if serverID, ok := l.DB.ServerIDBySyncID(parts[0]); ok {
			l.Vault.Delete(stableSecretID(serverID, parts[1]))
			vaultChanged = true
			applied++
		}
	}
	for _, record := range deletions(KindForward) {
		if id, ok := l.DB.ForwardIDBySyncID(strings.TrimPrefix(record.ID, KindForward+":")); ok {
			if err := l.DB.DeleteForward(id); err != nil {
				warn(record, err)
				continue
			}
			applied++
		}
	}
	for _, record := range deletions(KindServer) {
		id, ok := l.DB.ServerIDBySyncID(strings.TrimPrefix(record.ID, KindServer+":"))
		if !ok {
			continue
		}
		server, err := l.DB.GetServerByID(id)
		if err != nil {
			warn(record, err)
			continue
		}
		if err := l.DB.DeleteServer(server.Alias); err != nil {
			warn(record, err)
			continue
		}
		for _, secretType := range secretTypes {
			l.Vault.Delete(stableSecretID(id, secretType))
			l.Vault.Delete(legacySecretID(server.Alias, secretType))
		}
		vaultChanged = true
		applied++
	}
	for _, record := range deletions(KindTemplate) {
		if err := l.DB.DeleteCommandTemplate(strings.TrimPrefix(record.ID, KindTemplate+":")); err != nil {
			warn(record, err)
			continue
		}
		applied++
	}
	for _, record := range deletions(KindTag) {
		if err := l.DB.DeleteTag(strings.TrimPrefix(record.ID, KindTag+":")); err != nil {
			warn(record, err)
			continue
		}
		applied++
	}
	for _, record := range deletions(KindGroup) {
		if err := l.DB.DeleteGroup(strings.TrimPrefix(record.ID, KindGroup+":")); err != nil {
			warn(record, err)
			continue
		}
		applied++
	}
	// Key files are never deleted: removing a private key from disk is not
	// something a sync should do on its own.

	if vaultChanged {
		if err := l.Vault.Save(); err != nil {
			return applied, warnings, fmt.Errorf("save vault: %w", err)
		}
	}
	return applied, warnings, nil
}

// serverModel converts synced data to a profile. Route hops to profiles are
// resolved to local IDs; without routes it builds the first-pass profile.
func (l *Local) serverModel(data ServerData, withRoute bool) *model.Server {
	server := &model.Server{
		Alias: data.Alias, DisplayName: data.DisplayName, Host: data.Host, Port: data.Port, User: data.User,
		AuthMethod: model.AuthMethod(data.AuthMethod), IdentityFile: data.IdentityFile, GroupName: data.Group,
		Notes: data.Notes, StartupCommand: data.StartupCommand, Tags: data.Tags,
	}
	if !withRoute {
		return server
	}
	for _, hop := range data.Route {
		switch {
		case hop.Raw != "":
			server.Route.Hops = append(server.Route.Hops, model.RouteHop{Raw: hop.Raw})
		default:
			if id, ok := l.DB.ServerIDBySyncID(hop.Server); ok {
				server.Route.Hops = append(server.Route.Hops, model.RouteHop{ServerID: id, IsProfile: true})
			} else if hop.Alias != "" {
				// The hop profile is not on this device; keep the route
				// usable as a raw jump target.
				server.Route.Hops = append(server.Route.Hops, model.RouteHop{Raw: hop.Alias})
			}
		}
	}
	return server
}

// writeKey creates a synced key file when it is missing. An existing file is
// never overwritten: if it differs, the local key is kept and reported.
func (l *Local) writeKey(data KeyData) (bool, error) {
	full := l.expand(data.Path)
	existing, err := os.ReadFile(full)
	switch {
	case err == nil && string(existing) == string(data.Private):
		return false, nil
	case err == nil:
		return false, fmt.Errorf("a different key already exists here; the local key was kept")
	case !errors.Is(err, os.ErrNotExist):
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
		return false, err
	}
	if err := os.WriteFile(full, data.Private, 0o600); err != nil {
		return false, err
	}
	if len(data.Public) > 0 {
		if _, err := os.Stat(full + ".pub"); errors.Is(err, os.ErrNotExist) {
			_ = os.WriteFile(full+".pub", data.Public, 0o644)
		}
	}
	return true, nil
}

func decode(record Record, target any) error {
	if err := json.Unmarshal(record.Data, target); err != nil {
		return fmt.Errorf("unreadable record: %w", err)
	}
	return nil
}

// describe names a record in a warning without exposing secret values.
func describe(record Record) string {
	switch record.Kind {
	case KindServer:
		var data ServerData
		if json.Unmarshal(record.Data, &data) == nil && data.Alias != "" {
			return "profile " + data.Alias
		}
	case KindKey:
		return "key " + strings.TrimPrefix(record.ID, KindKey+":")
	case KindSecret:
		return "secret"
	}
	return strings.Replace(record.ID, ":", " ", 1)
}
