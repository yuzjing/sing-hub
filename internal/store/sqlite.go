// internal/store/sqlite.go
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"sing-hub/internal/model"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
	mu sync.Mutex
}

func New(dbPath string) (*Store, error) {
	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite failed: %w", err)
	}

	db.SetMaxOpenConns(1)

	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	ddl := `
	CREATE TABLE IF NOT EXISTS components (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		category TEXT NOT NULL DEFAULT 'routing',
		scope_tag TEXT NOT NULL DEFAULT 'universal',
		mount_path TEXT NOT NULL,
		strategy TEXT NOT NULL,
		payload_tmpl TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS profiles (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		token TEXT NOT NULL UNIQUE,
		description TEXT NOT NULL DEFAULT '',
		sing_box_version TEXT NOT NULL DEFAULT '',
		enable_subscribe INTEGER NOT NULL DEFAULT 1,
		enable_ping INTEGER NOT NULL DEFAULT 1,
		enable_push INTEGER NOT NULL DEFAULT 0,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS profile_components (
		profile_id TEXT NOT NULL,
		component_id TEXT NOT NULL,
		order_index INTEGER NOT NULL DEFAULT 0,
		params_json TEXT NOT NULL DEFAULT '{}',
		PRIMARY KEY (profile_id, component_id),
		FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE,
		FOREIGN KEY (component_id) REFERENCES components(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_profile_order ON profile_components(profile_id, order_index ASC);

	CREATE TABLE IF NOT EXISTS devices (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		endpoints_json TEXT NOT NULL DEFAULT '[]',
		ssh_host TEXT NOT NULL DEFAULT '',
		target_profile_id TEXT NOT NULL DEFAULT '',
		auth_type TEXT NOT NULL DEFAULT 'key_path',
		auth_secret TEXT NOT NULL DEFAULT '',
		key_path TEXT NOT NULL DEFAULT '',
		ssh_user TEXT NOT NULL DEFAULT 'root',
		ssh_port INTEGER NOT NULL DEFAULT 22,
		remote_path TEXT NOT NULL DEFAULT '~/.config/sing-box/config.json',
		tags_json TEXT NOT NULL DEFAULT '[]',
		enable_ping INTEGER NOT NULL DEFAULT 1,
		enable_push INTEGER NOT NULL DEFAULT 1,
		description TEXT NOT NULL DEFAULT '',
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);
	`
	_, err := s.db.ExecContext(ctx, ddl)
	if err != nil {
		return err
	}

	// 针对旧数据库自动无损热迁移，补全全部缺失列
	patchQueries := []string{
		"ALTER TABLE devices ADD COLUMN endpoints_json TEXT NOT NULL DEFAULT '[]'",
		"ALTER TABLE devices ADD COLUMN ssh_host TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE devices ADD COLUMN target_profile_id TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE devices ADD COLUMN auth_type TEXT NOT NULL DEFAULT 'key_path'",
		"ALTER TABLE devices ADD COLUMN auth_secret TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE devices ADD COLUMN key_path TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE devices ADD COLUMN ssh_user TEXT NOT NULL DEFAULT 'root'",
		"ALTER TABLE devices ADD COLUMN ssh_port INTEGER NOT NULL DEFAULT 22",
		"ALTER TABLE devices ADD COLUMN remote_path TEXT NOT NULL DEFAULT '~/.config/sing-box/config.json'",
		"ALTER TABLE devices ADD COLUMN tags_json TEXT NOT NULL DEFAULT '[]'",
		"ALTER TABLE devices ADD COLUMN enable_ping INTEGER NOT NULL DEFAULT 1",
		"ALTER TABLE devices ADD COLUMN enable_push INTEGER NOT NULL DEFAULT 1",
		"ALTER TABLE components ADD COLUMN scope_tag TEXT NOT NULL DEFAULT 'universal'",
		"ALTER TABLE profiles ADD COLUMN sing_box_version TEXT NOT NULL DEFAULT ''",
	}

	for _, q := range patchQueries {
		_, _ = s.db.ExecContext(ctx, q)
	}

	return nil
}

// ==================== Settings ====================

func (s *Store) GetSetting(ctx context.Context, key string, fallback string) string {
	var val string
	row := s.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = ?", key)
	if err := row.Scan(&val); err != nil {
		return fallback
	}
	return val
}

func (s *Store) SetSetting(ctx context.Context, key string, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	query := "INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value"
	_, err := s.db.ExecContext(ctx, query, key, value)
	return err
}

// ==================== SQL Dump & Restore ====================

func (s *Store) DumpSQL(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("-- Sing-Hub Database Dump (Text SQL)\n\n")

	cRows, err := s.db.QueryContext(ctx, "SELECT id, name, category, scope_tag, mount_path, strategy, payload_tmpl, description, updated_at FROM components")
	if err == nil {
		for cRows.Next() {
			var c model.Component
			_ = cRows.Scan(&c.ID, &c.Name, &c.Category, &c.ScopeTag, &c.MountPath, &c.Strategy, &c.PayloadTmpl, &c.Description, &c.UpdatedAt)
			sb.WriteString(fmt.Sprintf("INSERT OR REPLACE INTO components VALUES (%q, %q, %q, %q, %q, %q, %q, %q, %q);\n",
				c.ID, c.Name, c.Category, c.ScopeTag, c.MountPath, c.Strategy, c.PayloadTmpl, c.Description, c.UpdatedAt.Format(time.RFC3339)))
		}
		cRows.Close()
	}

	pRows, err := s.db.QueryContext(ctx, "SELECT id, name, token, description, sing_box_version, enable_subscribe, enable_ping, enable_push, updated_at FROM profiles")
	if err == nil {
		for pRows.Next() {
			var id, name, token, desc, sbv string
			var sub, ping, push int
			var u time.Time
			_ = pRows.Scan(&id, &name, &token, &desc, &sbv, &sub, &ping, &push, &u)
			sb.WriteString(fmt.Sprintf("INSERT OR REPLACE INTO profiles VALUES (%q, %q, %q, %q, %q, %d, %d, %d, %q);\n",
				id, name, token, desc, sbv, sub, ping, push, u.Format(time.RFC3339)))
		}
		pRows.Close()
	}

	bRows, err := s.db.QueryContext(ctx, "SELECT profile_id, component_id, order_index, params_json FROM profile_components")
	if err == nil {
		for bRows.Next() {
			var pid, cid, params string
			var order int
			_ = bRows.Scan(&pid, &cid, &order, &params)
			sb.WriteString(fmt.Sprintf("INSERT OR REPLACE INTO profile_components VALUES (%q, %q, %d, %q);\n", pid, cid, order, params))
		}
		bRows.Close()
	}

	dRows, err := s.db.QueryContext(ctx, "SELECT id, name, endpoints_json, ssh_host, target_profile_id, auth_type, auth_secret, key_path, ssh_user, ssh_port, remote_path, tags_json, enable_ping, enable_push, description, updated_at FROM devices")
	if err == nil {
		for dRows.Next() {
			var id, name, eps, shost, prof, authT, authS, kp, u, rem, tags, desc string
			var port, ping, push int
			var up time.Time
			_ = dRows.Scan(&id, &name, &eps, &shost, &prof, &authT, &authS, &kp, &u, &port, &rem, &tags, &ping, &push, &desc, &up)
			sb.WriteString(fmt.Sprintf("INSERT OR REPLACE INTO devices VALUES (%q, %q, %q, %q, %q, %q, %q, %q, %q, %d, %q, %q, %d, %d, %q, %q);\n",
				id, name, eps, shost, prof, authT, authS, kp, u, port, rem, tags, ping, push, desc, up.Format(time.RFC3339)))
		}
		dRows.Close()
	}

	return sb.String(), nil
}

func (s *Store) RestoreSQL(ctx context.Context, sqlScript string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	statements := strings.Split(sqlScript, ";")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("restore error: %w (stmt: %s)", err, stmt)
		}
	}
	return tx.Commit()
}

// ==================== Components ====================

func (s *Store) ListComponents(ctx context.Context) ([]model.Component, error) {
	list := make([]model.Component, 0)
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, category, scope_tag, mount_path, strategy, payload_tmpl, description, updated_at FROM components ORDER BY category ASC, name ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var c model.Component
		if err := rows.Scan(&c.ID, &c.Name, &c.Category, &c.ScopeTag, &c.MountPath, &c.Strategy, &c.PayloadTmpl, &c.Description, &c.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (s *Store) SaveComponent(ctx context.Context, c model.Component) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO components (id, name, category, scope_tag, mount_path, strategy, payload_tmpl, description, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		name = excluded.name,
		category = excluded.category,
		scope_tag = excluded.scope_tag,
		mount_path = excluded.mount_path,
		strategy = excluded.strategy,
		payload_tmpl = excluded.payload_tmpl,
		description = excluded.description,
		updated_at = excluded.updated_at;
	`
	_, err := s.db.ExecContext(ctx, query, c.ID, c.Name, c.Category, c.ScopeTag, c.MountPath, c.Strategy, c.PayloadTmpl, c.Description, time.Now())
	return err
}

func (s *Store) DeleteComponent(ctx context.Context, id string) error {
	return s.BatchDeleteComponents(ctx, []string{id})
}

func (s *Store) BatchDeleteComponents(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	query := fmt.Sprintf("DELETE FROM components WHERE id IN (%s)", quoteSlice(ids))
	_, err := s.db.ExecContext(ctx, query)
	return err
}

// ==================== Profiles ====================

func (s *Store) ListProfiles(ctx context.Context) ([]model.Profile, error) {
	list := make([]model.Profile, 0)
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, token, description, sing_box_version, enable_subscribe, enable_ping, enable_push, updated_at FROM profiles ORDER BY name ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var p model.Profile
		var sub, ping, push int
		if err := rows.Scan(&p.ID, &p.Name, &p.Token, &p.Description, &p.SingBoxVersion, &sub, &ping, &push, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.Capabilities = model.ProfileCapabilities{
			EnableSubscribe: sub == 1,
			EnablePing:      ping == 1,
			EnablePush:      push == 1,
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (s *Store) GetProfile(ctx context.Context, id string) (*model.Profile, error) {
	var p model.Profile
	var sub, ping, push int
	row := s.db.QueryRowContext(ctx, "SELECT id, name, token, description, sing_box_version, enable_subscribe, enable_ping, enable_push, updated_at FROM profiles WHERE id = ?", id)
	if err := row.Scan(&p.ID, &p.Name, &p.Token, &p.Description, &p.SingBoxVersion, &sub, &ping, &push, &p.UpdatedAt); err != nil {
		return nil, err
	}
	p.Capabilities = model.ProfileCapabilities{
		EnableSubscribe: sub == 1,
		EnablePing:      ping == 1,
		EnablePush:      push == 1,
	}

	p.Components = make([]model.ProfileComponentBinding, 0)
	bRows, err := s.db.QueryContext(ctx, "SELECT component_id, order_index, params_json FROM profile_components WHERE profile_id = ? ORDER BY order_index ASC", id)
	if err != nil {
		return nil, err
	}
	defer bRows.Close()

	for bRows.Next() {
		var b model.ProfileComponentBinding
		if err := bRows.Scan(&b.ComponentID, &b.OrderIndex, &b.ParamsJSON); err != nil {
			return nil, err
		}
		p.Components = append(p.Components, b)
	}
	return &p, bRows.Err()
}

func (s *Store) GetProfileByToken(ctx context.Context, token string) (*model.Profile, error) {
	var id string
	row := s.db.QueryRowContext(ctx, "SELECT id FROM profiles WHERE token = ?", token)
	if err := row.Scan(&id); err != nil {
		return nil, err
	}
	return s.GetProfile(ctx, id)
}

func (s *Store) SaveProfile(ctx context.Context, p model.Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	sub := 0
	if p.Capabilities.EnableSubscribe {
		sub = 1
	}
	ping := 0
	if p.Capabilities.EnablePing {
		ping = 1
	}
	push := 0
	if p.Capabilities.EnablePush {
		push = 1
	}

	query := `
	INSERT INTO profiles (id, name, token, description, sing_box_version, enable_subscribe, enable_ping, enable_push, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		name = excluded.name,
		token = excluded.token,
		description = excluded.description,
		sing_box_version = excluded.sing_box_version,
		enable_subscribe = excluded.enable_subscribe,
		enable_ping = excluded.enable_ping,
		enable_push = excluded.enable_push,
		updated_at = excluded.updated_at;
	`
	if _, err := tx.ExecContext(ctx, query, p.ID, p.Name, p.Token, p.Description, p.SingBoxVersion, sub, ping, push, time.Now()); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM profile_components WHERE profile_id = ?", p.ID); err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx, "INSERT INTO profile_components (profile_id, component_id, order_index, params_json) VALUES (?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, c := range p.Components {
		params := c.ParamsJSON
		if params == "" {
			params = "{}"
		}
		if _, err := stmt.ExecContext(ctx, p.ID, c.ComponentID, c.OrderIndex, params); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) DeleteProfile(ctx context.Context, id string) error {
	return s.BatchDeleteProfiles(ctx, []string{id})
}

func (s *Store) BatchDeleteProfiles(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	query := fmt.Sprintf("DELETE FROM profiles WHERE id IN (%s)", quoteSlice(ids))
	_, err := s.db.ExecContext(ctx, query)
	return err
}

// ==================== Devices (对齐字段解决 500 报错) ====================

func (s *Store) ListDevices(ctx context.Context) ([]model.Device, error) {
	list := make([]model.Device, 0)
	// 严格对齐 16 个字段
	query := "SELECT id, name, endpoints_json, ssh_host, target_profile_id, auth_type, auth_secret, key_path, ssh_user, ssh_port, remote_path, tags_json, enable_ping, enable_push, description, updated_at FROM devices ORDER BY name ASC"
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var d model.Device
		var epsJSON, tagsJSON string
		var ping, push int
		if err := rows.Scan(
			&d.ID, &d.Name, &epsJSON, &d.SSHHost, &d.TargetProfileID, &d.AuthType, &d.AuthSecret,
			&d.KeyPath, &d.SSHUser, &d.SSHPort, &d.RemotePath, &tagsJSON,
			&ping, &push, &d.Description, &d.UpdatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(epsJSON), &d.Endpoints)
		if d.Endpoints == nil {
			d.Endpoints = make([]string, 0)
		}
		_ = json.Unmarshal([]byte(tagsJSON), &d.Tags)
		if d.Tags == nil {
			d.Tags = make([]string, 0)
		}
		d.EnablePing = ping == 1
		d.EnablePush = push == 1
		list = append(list, d)
	}
	return list, rows.Err()
}

func (s *Store) SaveDevice(ctx context.Context, d model.Device) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	epsBytes, _ := json.Marshal(d.Endpoints)
	tagsBytes, _ := json.Marshal(d.Tags)
	ping, push := 0, 0
	if d.EnablePing {
		ping = 1
	}
	if d.EnablePush {
		push = 1
	}

	query := `
	INSERT INTO devices (id, name, endpoints_json, ssh_host, target_profile_id, auth_type, auth_secret, key_path, ssh_user, ssh_port, remote_path, tags_json, enable_ping, enable_push, description, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		name = excluded.name,
		endpoints_json = excluded.endpoints_json,
		ssh_host = excluded.ssh_host,
		target_profile_id = excluded.target_profile_id,
		auth_type = excluded.auth_type,
		auth_secret = excluded.auth_secret,
		key_path = excluded.key_path,
		ssh_user = excluded.ssh_user,
		ssh_port = excluded.ssh_port,
		remote_path = excluded.remote_path,
		tags_json = excluded.tags_json,
		enable_ping = excluded.enable_ping,
		enable_push = excluded.enable_push,
		description = excluded.description,
		updated_at = excluded.updated_at;
	`
	_, err := s.db.ExecContext(ctx, query,
		d.ID, d.Name, string(epsBytes), d.SSHHost, d.TargetProfileID, d.AuthType, d.AuthSecret,
		d.KeyPath, d.SSHUser, d.SSHPort, d.RemotePath, string(tagsBytes),
		ping, push, d.Description, time.Now(),
	)
	return err
}

func (s *Store) DeleteDevice(ctx context.Context, id string) error {
	return s.BatchDeleteDevices(ctx, []string{id})
}

func (s *Store) BatchDeleteDevices(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	query := fmt.Sprintf("DELETE FROM devices WHERE id IN (%s)", quoteSlice(ids))
	_, err := s.db.ExecContext(ctx, query)
	return err
}

func (s *Store) GetResolvedProfileBindings(ctx context.Context, profileID string) ([]struct {
	Component  model.Component
	ParamsJSON string
}, error) {
	query := `
	SELECT c.id, c.name, c.category, c.scope_tag, c.mount_path, c.strategy, c.payload_tmpl, c.description, c.updated_at, pc.params_json
	FROM components c
	JOIN profile_components pc ON c.id = pc.component_id
	WHERE pc.profile_id = ?
	ORDER BY pc.order_index ASC;
	`
	rows, err := s.db.QueryContext(ctx, query, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []struct {
		Component  model.Component
		ParamsJSON string
	}

	for rows.Next() {
		var item struct {
			Component  model.Component
			ParamsJSON string
		}
		if err := rows.Scan(
			&item.Component.ID, &item.Component.Name, &item.Component.Category,
			&item.Component.ScopeTag, &item.Component.MountPath, &item.Component.Strategy,
			&item.Component.PayloadTmpl, &item.Component.Description, &item.Component.UpdatedAt,
			&item.ParamsJSON,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func quoteSlice(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return strings.Join(quoted, ",")
}
