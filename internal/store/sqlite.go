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
		sing_box_version TEXT NOT NULL DEFAULT 'universal',
		mount_path TEXT NOT NULL,
		strategy TEXT NOT NULL,
		payload_tmpl TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS modules (
		id TEXT PRIMARY KEY,
		template_id TEXT NOT NULL,
		name TEXT NOT NULL,
		category TEXT NOT NULL DEFAULT 'io',
		params_json TEXT NOT NULL DEFAULT '{}',
		tags_json TEXT NOT NULL DEFAULT '[]',
		description TEXT NOT NULL DEFAULT '',
		updated_at DATETIME NOT NULL,
		FOREIGN KEY (template_id) REFERENCES components(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS profiles (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		token TEXT NOT NULL UNIQUE,
		tags_json TEXT NOT NULL DEFAULT '[]',
		description TEXT NOT NULL DEFAULT '',
		sing_box_version TEXT NOT NULL DEFAULT '',
		enable_subscribe INTEGER NOT NULL DEFAULT 1,
		enable_ping INTEGER NOT NULL DEFAULT 1,
		enable_push INTEGER NOT NULL DEFAULT 0,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS profile_modules (
		id TEXT PRIMARY KEY,
		profile_id TEXT NOT NULL,
		module_id TEXT NOT NULL,
		order_index INTEGER NOT NULL DEFAULT 0,
		FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE,
		FOREIGN KEY (module_id) REFERENCES modules(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_profile_modules_order ON profile_modules(profile_id, order_index ASC);

	CREATE TABLE IF NOT EXISTS devices (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		endpoints_json TEXT NOT NULL DEFAULT '[]',
		props_json TEXT NOT NULL DEFAULT '[]',
		ssh_host TEXT NOT NULL DEFAULT '',
		target_profile_id TEXT NOT NULL DEFAULT '',
		auth_type TEXT NOT NULL DEFAULT 'key_path',
		auth_secret TEXT NOT NULL DEFAULT '',
		key_path TEXT NOT NULL DEFAULT '',
		ssh_user TEXT NOT NULL DEFAULT 'root',
		ssh_port INTEGER NOT NULL DEFAULT 22,
		remote_path TEXT NOT NULL DEFAULT '~/.config/sing-box/config.json',
		deploy_strategy TEXT NOT NULL DEFAULT 'systemd',
		service_name TEXT NOT NULL DEFAULT 'sing-box',
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

	_, _ = s.db.ExecContext(ctx, "ALTER TABLE components ADD COLUMN sing_box_version TEXT NOT NULL DEFAULT 'universal'")
	_, _ = s.db.ExecContext(ctx, "ALTER TABLE modules ADD COLUMN tags_json TEXT NOT NULL DEFAULT '[]'")
	_, _ = s.db.ExecContext(ctx, "ALTER TABLE profiles ADD COLUMN tags_json TEXT NOT NULL DEFAULT '[]'")
	_, _ = s.db.ExecContext(ctx, "ALTER TABLE devices ADD COLUMN props_json TEXT NOT NULL DEFAULT '[]'")
	_, _ = s.db.ExecContext(ctx, "ALTER TABLE devices ADD COLUMN deploy_strategy TEXT NOT NULL DEFAULT 'systemd'")
	_, _ = s.db.ExecContext(ctx, "ALTER TABLE devices ADD COLUMN service_name TEXT NOT NULL DEFAULT 'sing-box'")

	_ = s.migrateOldComponentsToModules(ctx)

	return nil
}

func (s *Store) migrateOldComponentsToModules(ctx context.Context) error {
	var count int
	_ = s.db.QueryRowContext(ctx, "SELECT count(*) FROM modules").Scan(&count)
	if count > 0 {
		return nil
	}

	comps, err := s.ListComponents(ctx)
	if err != nil || len(comps) == 0 {
		return nil
	}

	for _, c := range comps {
		mod := model.Module{
			ID:          "mod-" + c.ID,
			TemplateID:  c.ID,
			Name:        c.Name,
			Category:    c.Category,
			ParamsJSON:  "{}",
			Description: c.Description,
			UpdatedAt:   time.Now(),
		}
		_ = s.SaveModule(ctx, mod)
	}

	rows, err := s.db.QueryContext(ctx, "SELECT profile_id, component_id, order_index, params_json FROM profile_components")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var pid, cid, params string
			var order int
			if err := rows.Scan(&pid, &cid, &order, &params); err == nil {
				mid := "mod-" + cid
				_, _ = s.db.ExecContext(ctx, "UPDATE modules SET params_json = ? WHERE id = ?", params, mid)
				bindID := fmt.Sprintf("bind-%s-%s-%d", pid, mid, order)
				_, _ = s.db.ExecContext(ctx, "INSERT OR REPLACE INTO profile_modules (id, profile_id, module_id, order_index) VALUES (?, ?, ?, ?)",
					bindID, pid, mid, order)
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
	}
	return nil
}

// ==================== Modules CRUD ====================

func (s *Store) ListModules(ctx context.Context) ([]model.Module, error) {
	list := make([]model.Module, 0)
	query := `
	SELECT m.id, m.template_id, m.name, m.category, m.params_json, m.tags_json, m.description, m.updated_at,
	       c.name, c.mount_path, c.strategy, c.scope_tag, c.sing_box_version
	FROM modules m
	LEFT JOIN components c ON m.template_id = c.id
	ORDER BY m.category ASC, m.name ASC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var m model.Module
		var tagsJSON string
		var cName, mountPath, strategy, scopeTag, sbVer sql.NullString
		if err := rows.Scan(&m.ID, &m.TemplateID, &m.Name, &m.Category, &m.ParamsJSON, &tagsJSON, &m.Description, &m.UpdatedAt,
			&cName, &mountPath, &strategy, &scopeTag, &sbVer); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(tagsJSON), &m.Tags)
		if cName.Valid {
			m.Template = &model.Component{
				ID:             m.TemplateID,
				Name:           cName.String,
				MountPath:      mountPath.String,
				Strategy:       model.Strategy(strategy.String),
				ScopeTag:       scopeTag.String,
				SingBoxVersion: sbVer.String,
			}
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (s *Store) GetModule(ctx context.Context, id string) (*model.Module, error) {
	var m model.Module
	var tagsJSON string
	row := s.db.QueryRowContext(ctx, "SELECT id, template_id, name, category, params_json, tags_json, description, updated_at FROM modules WHERE id = ?", id)
	if err := row.Scan(&m.ID, &m.TemplateID, &m.Name, &m.Category, &m.ParamsJSON, &tagsJSON, &m.Description, &m.UpdatedAt); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(tagsJSON), &m.Tags)
	return &m, nil
}

func (s *Store) SaveModule(ctx context.Context, m model.Module) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tagsB, _ := json.Marshal(m.Tags)

	query := `
	INSERT INTO modules (id, template_id, name, category, params_json, tags_json, description, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		template_id = excluded.template_id,
		name = excluded.name,
		category = excluded.category,
		params_json = excluded.params_json,
		tags_json = excluded.tags_json,
		description = excluded.description,
		updated_at = excluded.updated_at;
	`
	_, err := s.db.ExecContext(ctx, query, m.ID, m.TemplateID, m.Name, m.Category, m.ParamsJSON, string(tagsB), m.Description, time.Now())
	return err
}

func (s *Store) DeleteModule(ctx context.Context, id string) error {
	return s.BatchDeleteModules(ctx, []string{id})
}

func (s *Store) BatchDeleteModules(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	query := fmt.Sprintf("DELETE FROM modules WHERE id IN (%s)", quoteSlice(ids))
	_, err := s.db.ExecContext(ctx, query)
	return err
}

// ==================== Components CRUD ====================

func (s *Store) ListComponents(ctx context.Context) ([]model.Component, error) {
	list := make([]model.Component, 0)
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, category, scope_tag, sing_box_version, mount_path, strategy, payload_tmpl, description, updated_at FROM components ORDER BY category ASC, name ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var c model.Component
		if err := rows.Scan(&c.ID, &c.Name, &c.Category, &c.ScopeTag, &c.SingBoxVersion, &c.MountPath, &c.Strategy, &c.PayloadTmpl, &c.Description, &c.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (s *Store) GetComponent(ctx context.Context, id string) (*model.Component, error) {
	var c model.Component
	row := s.db.QueryRowContext(ctx, "SELECT id, name, category, scope_tag, sing_box_version, mount_path, strategy, payload_tmpl, description, updated_at FROM components WHERE id = ?", id)
	if err := row.Scan(&c.ID, &c.Name, &c.Category, &c.ScopeTag, &c.SingBoxVersion, &c.MountPath, &c.Strategy, &c.PayloadTmpl, &c.Description, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) SaveComponent(ctx context.Context, c model.Component) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO components (id, name, category, scope_tag, sing_box_version, mount_path, strategy, payload_tmpl, description, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		name = excluded.name,
		category = excluded.category,
		scope_tag = excluded.scope_tag,
		sing_box_version = excluded.sing_box_version,
		mount_path = excluded.mount_path,
		strategy = excluded.strategy,
		payload_tmpl = excluded.payload_tmpl,
		description = excluded.description,
		updated_at = excluded.updated_at;
	`
	_, err := s.db.ExecContext(ctx, query, c.ID, c.Name, c.Category, c.ScopeTag, c.SingBoxVersion, c.MountPath, c.Strategy, c.PayloadTmpl, c.Description, time.Now())
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

// ==================== Profiles CRUD ====================

func (s *Store) ListProfiles(ctx context.Context) ([]model.Profile, error) {
	list := make([]model.Profile, 0)
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, token, tags_json, description, sing_box_version, enable_subscribe, enable_ping, enable_push, updated_at FROM profiles ORDER BY name ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var p model.Profile
		var tagsJSON string
		var sub, ping, push int
		if err := rows.Scan(&p.ID, &p.Name, &p.Token, &tagsJSON, &p.Description, &p.SingBoxVersion, &sub, &ping, &push, &p.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(tagsJSON), &p.Tags)
		p.Capabilities = model.ProfileCapabilities{
			EnableSubscribe: sub == 1,
			EnablePing:      ping == 1,
			EnablePush:      push == 1,
		}
		p.Modules = make([]model.ProfileModuleBinding, 0)
		list = append(list, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	bRows, err := s.db.QueryContext(ctx, "SELECT id, profile_id, module_id, order_index FROM profile_modules ORDER BY order_index ASC")
	if err == nil {
		defer bRows.Close()
		bindMap := make(map[string][]model.ProfileModuleBinding)
		for bRows.Next() {
			var b model.ProfileModuleBinding
			if err := bRows.Scan(&b.ID, &b.ProfileID, &b.ModuleID, &b.OrderIndex); err == nil {
				bindMap[b.ProfileID] = append(bindMap[b.ProfileID], b)
			}
		}
		_ = bRows.Err()
		for i := range list {
			if binds, ok := bindMap[list[i].ID]; ok {
				list[i].Modules = binds
			}
		}
	}

	return list, nil
}

func (s *Store) GetProfile(ctx context.Context, id string) (*model.Profile, error) {
	var p model.Profile
	var tagsJSON string
	var sub, ping, push int
	row := s.db.QueryRowContext(ctx, "SELECT id, name, token, tags_json, description, sing_box_version, enable_subscribe, enable_ping, enable_push, updated_at FROM profiles WHERE id = ?", id)
	if err := row.Scan(&p.ID, &p.Name, &p.Token, &tagsJSON, &p.Description, &p.SingBoxVersion, &sub, &ping, &push, &p.UpdatedAt); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(tagsJSON), &p.Tags)
	p.Capabilities = model.ProfileCapabilities{
		EnableSubscribe: sub == 1,
		EnablePing:      ping == 1,
		EnablePush:      push == 1,
	}

	p.Modules = make([]model.ProfileModuleBinding, 0)
	bRows, err := s.db.QueryContext(ctx, "SELECT id, module_id, order_index FROM profile_modules WHERE profile_id = ? ORDER BY order_index ASC", id)
	if err != nil {
		return nil, err
	}
	defer bRows.Close()

	for bRows.Next() {
		var b model.ProfileModuleBinding
		b.ProfileID = id
		if err := bRows.Scan(&b.ID, &b.ModuleID, &b.OrderIndex); err != nil {
			return nil, err
		}
		p.Modules = append(p.Modules, b)
	}
	if err := bRows.Err(); err != nil {
		return nil, err
	}
	return &p, nil
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

	tagsB, _ := json.Marshal(p.Tags)

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
	INSERT INTO profiles (id, name, token, tags_json, description, sing_box_version, enable_subscribe, enable_ping, enable_push, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		name = excluded.name,
		token = excluded.token,
		tags_json = excluded.tags_json,
		description = excluded.description,
		sing_box_version = excluded.sing_box_version,
		enable_subscribe = excluded.enable_subscribe,
		enable_ping = excluded.enable_ping,
		enable_push = excluded.enable_push,
		updated_at = excluded.updated_at;
	`
	if _, err := tx.ExecContext(ctx, query, p.ID, p.Name, p.Token, string(tagsB), p.Description, p.SingBoxVersion, sub, ping, push, time.Now()); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM profile_modules WHERE profile_id = ?", p.ID); err != nil {
		return err
	}

	for idx, b := range p.Modules {
		bindID := b.ID
		if strings.TrimSpace(bindID) == "" {
			bindID = fmt.Sprintf("bind-%s-%s-%d", p.ID, b.ModuleID, idx)
		}
		order := b.OrderIndex
		if order == 0 {
			order = idx + 1
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO profile_modules (id, profile_id, module_id, order_index) VALUES (?, ?, ?, ?)",
			bindID, p.ID, b.ModuleID, order); err != nil {
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

type ResolvedModuleBinding struct {
	Binding  model.ProfileModuleBinding
	Module   model.Module
	Template model.Component
}

func (s *Store) GetResolvedProfileModules(ctx context.Context, profileID string) ([]ResolvedModuleBinding, error) {
	query := `
	SELECT pm.id, pm.profile_id, pm.module_id, pm.order_index,
	       COALESCE(m.id, ''), COALESCE(m.template_id, ''), COALESCE(m.name, '未命名积木'), COALESCE(m.category, 'io'), COALESCE(m.params_json, '{}'), COALESCE(m.description, ''), m.updated_at,
	       COALESCE(c.id, ''), COALESCE(c.name, '未命名母版'), COALESCE(c.category, 'io'), COALESCE(c.scope_tag, 'universal'), COALESCE(c.sing_box_version, 'universal'), COALESCE(c.mount_path, 'inbounds'), COALESCE(c.strategy, 'append'), COALESCE(c.payload_tmpl, '{}'), COALESCE(c.description, ''), c.updated_at
	FROM profile_modules pm
	LEFT JOIN modules m ON pm.module_id = m.id
	LEFT JOIN components c ON m.template_id = c.id
	WHERE pm.profile_id = ?
	ORDER BY pm.order_index ASC
	`
	rows, err := s.db.QueryContext(ctx, query, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ResolvedModuleBinding
	for rows.Next() {
		var item ResolvedModuleBinding
		var mUpdatedAt, cUpdatedAt sql.NullTime

		if err := rows.Scan(
			&item.Binding.ID, &item.Binding.ProfileID, &item.Binding.ModuleID, &item.Binding.OrderIndex,
			&item.Module.ID, &item.Module.TemplateID, &item.Module.Name, &item.Module.Category, &item.Module.ParamsJSON, &item.Module.Description, &mUpdatedAt,
			&item.Template.ID, &item.Template.Name, &item.Template.Category, &item.Template.ScopeTag, &item.Template.SingBoxVersion, &item.Template.MountPath, &item.Template.Strategy, &item.Template.PayloadTmpl, &item.Template.Description, &cUpdatedAt,
		); err != nil {
			return nil, err
		}

		if mUpdatedAt.Valid {
			item.Module.UpdatedAt = mUpdatedAt.Time
		} else {
			item.Module.UpdatedAt = time.Now()
		}

		if cUpdatedAt.Valid {
			item.Template.UpdatedAt = cUpdatedAt.Time
		} else {
			item.Template.UpdatedAt = time.Now()
		}

		if item.Template.ID == "" {
			item.Template = model.Component{
				ID:          item.Module.TemplateID,
				Name:        item.Module.Name,
				Category:    item.Module.Category,
				MountPath:   "inbounds",
				Strategy:    model.StrategyAppend,
				PayloadTmpl: "{\n  \"outbound\": \"direct\"\n}",
			}
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// ==================== Devices CRUD ====================

func (s *Store) ListDevices(ctx context.Context) ([]model.Device, error) {
	list := make([]model.Device, 0)
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, endpoints_json, props_json, ssh_host, target_profile_id, auth_type, auth_secret, key_path, ssh_user, ssh_port, remote_path, deploy_strategy, service_name, tags_json, enable_ping, enable_push, description, updated_at FROM devices ORDER BY name ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var d model.Device
		var epsJSON, propsJSON, tagsJSON string
		var ping, push int
		if err := rows.Scan(&d.ID, &d.Name, &epsJSON, &propsJSON, &d.SSHHost, &d.TargetProfileID, &d.AuthType, &d.AuthSecret, &d.KeyPath, &d.SSHUser, &d.SSHPort, &d.RemotePath, &d.DeployStrategy, &d.ServiceName, &tagsJSON, &ping, &push, &d.Description, &d.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(epsJSON), &d.Endpoints)
		_ = json.Unmarshal([]byte(propsJSON), &d.Props)
		_ = json.Unmarshal([]byte(tagsJSON), &d.Tags)
		d.EnablePing = ping == 1
		d.EnablePush = push == 1
		list = append(list, d)
	}
	return list, rows.Err()
}

func (s *Store) GetDevice(ctx context.Context, id string) (*model.Device, error) {
	var d model.Device
	var epsJSON, propsJSON, tagsJSON string
	var ping, push int
	row := s.db.QueryRowContext(ctx, "SELECT id, name, endpoints_json, props_json, ssh_host, target_profile_id, auth_type, auth_secret, key_path, ssh_user, ssh_port, remote_path, deploy_strategy, service_name, tags_json, enable_ping, enable_push, description, updated_at FROM devices WHERE id = ?", id)
	if err := row.Scan(&d.ID, &d.Name, &epsJSON, &propsJSON, &d.SSHHost, &d.TargetProfileID, &d.AuthType, &d.AuthSecret, &d.KeyPath, &d.SSHUser, &d.SSHPort, &d.RemotePath, &d.DeployStrategy, &d.ServiceName, &tagsJSON, &ping, &push, &d.Description, &d.UpdatedAt); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(epsJSON), &d.Endpoints)
	_ = json.Unmarshal([]byte(propsJSON), &d.Props)
	_ = json.Unmarshal([]byte(tagsJSON), &d.Tags)
	d.EnablePing = ping == 1
	d.EnablePush = push == 1
	return &d, nil
}

func (s *Store) SaveDevice(ctx context.Context, d model.Device) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	epsB, _ := json.Marshal(d.Endpoints)
	propsB, _ := json.Marshal(d.Props)
	tagsB, _ := json.Marshal(d.Tags)
	ping := 0
	if d.EnablePing {
		ping = 1
	}
	push := 0
	if d.EnablePush {
		push = 1
	}

	if d.DeployStrategy == "" {
		d.DeployStrategy = "systemd"
	}
	if d.ServiceName == "" {
		d.ServiceName = "sing-box"
	}

	query := `
	INSERT INTO devices (id, name, endpoints_json, props_json, ssh_host, target_profile_id, auth_type, auth_secret, key_path, ssh_user, ssh_port, remote_path, deploy_strategy, service_name, tags_json, enable_ping, enable_push, description, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		name = excluded.name,
		endpoints_json = excluded.endpoints_json,
		props_json = excluded.props_json,
		ssh_host = excluded.ssh_host,
		target_profile_id = excluded.target_profile_id,
		auth_type = excluded.auth_type,
		auth_secret = excluded.auth_secret,
		key_path = excluded.key_path,
		ssh_user = excluded.ssh_user,
		ssh_port = excluded.ssh_port,
		remote_path = excluded.remote_path,
		deploy_strategy = excluded.deploy_strategy,
		service_name = excluded.service_name,
		tags_json = excluded.tags_json,
		enable_ping = excluded.enable_ping,
		enable_push = excluded.enable_push,
		description = excluded.description,
		updated_at = excluded.updated_at;
	`
	_, err := s.db.ExecContext(ctx, query, d.ID, d.Name, string(epsB), string(propsB), d.SSHHost, d.TargetProfileID, d.AuthType, d.AuthSecret, d.KeyPath, d.SSHUser, d.SSHPort, d.RemotePath, d.DeployStrategy, d.ServiceName, string(tagsB), ping, push, d.Description, time.Now())
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

// 关键修复：加入 devices 和 settings 的导出！
func (s *Store) DumpSQL(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("-- Sing-Hub Database Dump (Complete State)\n\n")

	cRows, err := s.db.QueryContext(ctx, "SELECT id, name, category, scope_tag, sing_box_version, mount_path, strategy, payload_tmpl, description, updated_at FROM components")
	if err == nil {
		defer cRows.Close()
		for cRows.Next() {
			var c model.Component
			_ = cRows.Scan(&c.ID, &c.Name, &c.Category, &c.ScopeTag, &c.SingBoxVersion, &c.MountPath, &c.Strategy, &c.PayloadTmpl, &c.Description, &c.UpdatedAt)
			sb.WriteString(fmt.Sprintf("INSERT OR REPLACE INTO components VALUES (%q, %q, %q, %q, %q, %q, %q, %q, %q, %q);\n",
				c.ID, c.Name, c.Category, c.ScopeTag, c.SingBoxVersion, c.MountPath, c.Strategy, c.PayloadTmpl, c.Description, c.UpdatedAt.Format(time.RFC3339)))
		}
		if err := cRows.Err(); err != nil {
			return "", err
		}
	}

	mRows, err := s.db.QueryContext(ctx, "SELECT id, template_id, name, category, params_json, tags_json, description, updated_at FROM modules")
	if err == nil {
		defer mRows.Close()
		for mRows.Next() {
			var m model.Module
			var tagsJSON string
			_ = mRows.Scan(&m.ID, &m.TemplateID, &m.Name, &m.Category, &m.ParamsJSON, &tagsJSON, &m.Description, &m.UpdatedAt)
			sb.WriteString(fmt.Sprintf("INSERT OR REPLACE INTO modules VALUES (%q, %q, %q, %q, %q, %q, %q, %q);\n",
				m.ID, m.TemplateID, m.Name, m.Category, m.ParamsJSON, tagsJSON, m.Description, m.UpdatedAt.Format(time.RFC3339)))
		}
		if err := mRows.Err(); err != nil {
			return "", err
		}
	}

	pRows, err := s.db.QueryContext(ctx, "SELECT id, name, token, tags_json, description, sing_box_version, enable_subscribe, enable_ping, enable_push, updated_at FROM profiles")
	if err == nil {
		defer pRows.Close()
		for pRows.Next() {
			var id, name, token, tagsJSON, desc, sbv string
			var sub, ping, push int
			var u time.Time
			_ = pRows.Scan(&id, &name, &token, &tagsJSON, &desc, &sbv, &sub, &ping, &push, &u)
			sb.WriteString(fmt.Sprintf("INSERT OR REPLACE INTO profiles VALUES (%q, %q, %q, %q, %q, %q, %d, %d, %d, %q);\n",
				id, name, token, tagsJSON, desc, sbv, sub, ping, push, u.Format(time.RFC3339)))
		}
		if err := pRows.Err(); err != nil {
			return "", err
		}
	}

	pmRows, err := s.db.QueryContext(ctx, "SELECT id, profile_id, module_id, order_index FROM profile_modules")
	if err == nil {
		defer pmRows.Close()
		for pmRows.Next() {
			var id, pid, mid string
			var order int
			_ = pmRows.Scan(&id, &pid, &mid, &order)
			sb.WriteString(fmt.Sprintf("INSERT OR REPLACE INTO profile_modules VALUES (%q, %q, %q, %d);\n", id, pid, mid, order))
		}
		if err := pmRows.Err(); err != nil {
			return "", err
		}
	}

	// 导出 devices
	dRows, err := s.db.QueryContext(ctx, "SELECT id, name, endpoints_json, props_json, ssh_host, target_profile_id, auth_type, auth_secret, key_path, ssh_user, ssh_port, remote_path, deploy_strategy, service_name, tags_json, enable_ping, enable_push, description, updated_at FROM devices")
	if err == nil {
		defer dRows.Close()
		for dRows.Next() {
			var d model.Device
			var eps, props, tags string
			var ping, push int
			var u time.Time
			_ = dRows.Scan(&d.ID, &d.Name, &eps, &props, &d.SSHHost, &d.TargetProfileID, &d.AuthType, &d.AuthSecret, &d.KeyPath, &d.SSHUser, &d.SSHPort, &d.RemotePath, &d.DeployStrategy, &d.ServiceName, &tags, &ping, &push, &u)
			sb.WriteString(fmt.Sprintf("INSERT OR REPLACE INTO devices VALUES (%q, %q, %q, %q, %q, %q, %q, %q, %q, %q, %d, %q, %q, %q, %q, %d, %d, %q, %q);\n",
				d.ID, d.Name, eps, props, d.SSHHost, d.TargetProfileID, d.AuthType, d.AuthSecret, d.KeyPath, d.SSHUser, d.SSHPort, d.RemotePath, d.DeployStrategy, d.ServiceName, tags, ping, push, d.Description, u.Format(time.RFC3339)))
		}
		if err := dRows.Err(); err != nil {
			return "", err
		}
	}

	// 导出 settings
	setRows, err := s.db.QueryContext(ctx, "SELECT key, value FROM settings")
	if err == nil {
		defer setRows.Close()
		for setRows.Next() {
			var k, v string
			_ = setRows.Scan(&k, &v)
			sb.WriteString(fmt.Sprintf("INSERT OR REPLACE INTO settings VALUES (%q, %q);\n", k, v))
		}
		if err := setRows.Err(); err != nil {
			return "", err
		}
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

func quoteSlice(ids []string) string {
	quoted := make([]string, len(ids))
	for i, id := range ids {
		quoted[i] = fmt.Sprintf("'%s'", strings.ReplaceAll(id, "'", "''"))
	}
	return strings.Join(quoted, ",")
}
