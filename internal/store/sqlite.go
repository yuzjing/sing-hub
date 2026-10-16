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

	-- 成品积木表
	CREATE TABLE IF NOT EXISTS modules (
		id TEXT PRIMARY KEY,
		template_id TEXT NOT NULL,
		name TEXT NOT NULL,
		category TEXT NOT NULL DEFAULT 'io',
		params_json TEXT NOT NULL DEFAULT '{}',
		description TEXT NOT NULL DEFAULT '',
		updated_at DATETIME NOT NULL,
		FOREIGN KEY (template_id) REFERENCES components(id) ON DELETE CASCADE
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

	-- 方案所挂载的成品模块表 (天然支持同模板多实例挂载)
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

	// 针对现有数据无损迁移：如果旧 profile_components 表存在，自动迁移至 modules 与 profile_modules
	_ = s.migrateOldComponentsToModules(ctx)

	return nil
}

func (s *Store) migrateOldComponentsToModules(ctx context.Context) error {
	var count int
	_ = s.db.QueryRowContext(ctx, "SELECT count(*) FROM modules").Scan(&count)
	if count > 0 {
		return nil
	}

	// 自动为已有 components 创建对应的默认 module
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

	// 尝试迁移 profile_components 关联
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

// ==================== Modules (第 2 层 CRUD) ====================

func (s *Store) ListModules(ctx context.Context) ([]model.Module, error) {
	list := make([]model.Module, 0)
	query := `
	SELECT m.id, m.template_id, m.name, m.category, m.params_json, m.description, m.updated_at,
	       c.name, c.mount_path, c.strategy, c.scope_tag
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
		var cName, mountPath, strategy, scopeTag sql.NullString
		if err := rows.Scan(&m.ID, &m.TemplateID, &m.Name, &m.Category, &m.ParamsJSON, &m.Description, &m.UpdatedAt,
			&cName, &mountPath, &strategy, &scopeTag); err != nil {
			return nil, err
		}
		if cName.Valid {
			m.Template = &model.Component{
				ID:        m.TemplateID,
				Name:      cName.String,
				MountPath: mountPath.String,
				Strategy:  model.Strategy(strategy.String),
				ScopeTag:  scopeTag.String,
			}
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (s *Store) GetModule(ctx context.Context, id string) (*model.Module, error) {
	var m model.Module
	row := s.db.QueryRowContext(ctx, "SELECT id, template_id, name, category, params_json, description, updated_at FROM modules WHERE id = ?", id)
	if err := row.Scan(&m.ID, &m.TemplateID, &m.Name, &m.Category, &m.ParamsJSON, &m.Description, &m.UpdatedAt); err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *Store) SaveModule(ctx context.Context, m model.Module) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO modules (id, template_id, name, category, params_json, description, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		template_id = excluded.template_id,
		name = excluded.name,
		category = excluded.category,
		params_json = excluded.params_json,
		description = excluded.description,
		updated_at = excluded.updated_at;
	`
	_, err := s.db.ExecContext(ctx, query, m.ID, m.TemplateID, m.Name, m.Category, m.ParamsJSON, m.Description, time.Now())
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

// ==================== Components (第 1 层 CRUD) ====================

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

func (s *Store) GetComponent(ctx context.Context, id string) (*model.Component, error) {
	var c model.Component
	row := s.db.QueryRowContext(ctx, "SELECT id, name, category, scope_tag, mount_path, strategy, payload_tmpl, description, updated_at FROM components WHERE id = ?", id)
	if err := row.Scan(&c.ID, &c.Name, &c.Category, &c.ScopeTag, &c.MountPath, &c.Strategy, &c.PayloadTmpl, &c.Description, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return &c, nil
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

// ==================== Profiles (第 3 层 CRUD) ====================

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
		p.Modules = make([]model.ProfileModuleBinding, 0)
		list = append(list, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 同时查出关联 modules，避免 loadProfiles() 冲刷掉大盘挂载状态
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
		for i := range list {
			if binds, ok := bindMap[list[i].ID]; ok {
				list[i].Modules = binds
			}
		}
	}

	if err := bRows.Err(); err != nil {
		return nil, err
	}

	return list, nil
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

	// 重新写入挂载的成品模块
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

// ==================== 关联解析查询 (用于编译引擎指针式挂载) ====================

type ResolvedModuleBinding struct {
	Binding  model.ProfileModuleBinding
	Module   model.Module
	Template model.Component // 保证取的是最新母版模板
}

func (s *Store) GetResolvedProfileModules(ctx context.Context, profileID string) ([]ResolvedModuleBinding, error) {
	query := `
	SELECT pm.id, pm.profile_id, pm.module_id, pm.order_index,
	       m.id, m.template_id, m.name, m.category, m.params_json, m.description, m.updated_at,
	       c.id, c.name, c.category, c.scope_tag, c.mount_path, c.strategy, c.payload_tmpl, c.description, c.updated_at
	FROM profile_modules pm
	JOIN modules m ON pm.module_id = m.id
	JOIN components c ON m.template_id = c.id
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
		if err := rows.Scan(
			&item.Binding.ID, &item.Binding.ProfileID, &item.Binding.ModuleID, &item.Binding.OrderIndex,
			&item.Module.ID, &item.Module.TemplateID, &item.Module.Name, &item.Module.Category, &item.Module.ParamsJSON, &item.Module.Description, &item.Module.UpdatedAt,
			&item.Template.ID, &item.Template.Name, &item.Template.Category, &item.Template.ScopeTag, &item.Template.MountPath, &item.Template.Strategy, &item.Template.PayloadTmpl, &item.Template.Description, &item.Template.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// ==================== Settings, SQL Dump & Restore, Devices 保持原样 ====================

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

func (s *Store) DumpSQL(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("-- Sing-Hub Database Dump (Text SQL)\n\n")

	cRows, err := s.db.QueryContext(ctx, "SELECT id, name, category, scope_tag, mount_path, strategy, payload_tmpl, description, updated_at FROM components")
	if err == nil {
		defer cRows.Close()
		for cRows.Next() {
			var c model.Component
			_ = cRows.Scan(&c.ID, &c.Name, &c.Category, &c.ScopeTag, &c.MountPath, &c.Strategy, &c.PayloadTmpl, &c.Description, &c.UpdatedAt)
			sb.WriteString(fmt.Sprintf("INSERT OR REPLACE INTO components VALUES (%q, %q, %q, %q, %q, %q, %q, %q, %q);\n",
				c.ID, c.Name, c.Category, c.ScopeTag, c.MountPath, c.Strategy, c.PayloadTmpl, c.Description, c.UpdatedAt.Format(time.RFC3339)))
		}
		if err := cRows.Err(); err != nil {
			return "", err
		}
	}

	mRows, err := s.db.QueryContext(ctx, "SELECT id, template_id, name, category, params_json, description, updated_at FROM modules")
	if err == nil {
		defer mRows.Close()
		for mRows.Next() {
			var m model.Module
			_ = mRows.Scan(&m.ID, &m.TemplateID, &m.Name, &m.Category, &m.ParamsJSON, &m.Description, &m.UpdatedAt)
			sb.WriteString(fmt.Sprintf("INSERT OR REPLACE INTO modules VALUES (%q, %q, %q, %q, %q, %q, %q);\n",
				m.ID, m.TemplateID, m.Name, m.Category, m.ParamsJSON, m.Description, m.UpdatedAt.Format(time.RFC3339)))
		}
		if err := mRows.Err(); err != nil {
			return "", err
		}
	}

	pRows, err := s.db.QueryContext(ctx, "SELECT id, name, token, description, sing_box_version, enable_subscribe, enable_ping, enable_push, updated_at FROM profiles")
	if err == nil {
		defer pRows.Close()
		for pRows.Next() {
			var id, name, token, desc, sbv string
			var sub, ping, push int
			var u time.Time
			_ = pRows.Scan(&id, &name, &token, &desc, &sbv, &sub, &ping, &push, &u)
			sb.WriteString(fmt.Sprintf("INSERT OR REPLACE INTO profiles VALUES (%q, %q, %q, %q, %q, %d, %d, %d, %q);\n",
				id, name, token, desc, sbv, sub, ping, push, u.Format(time.RFC3339)))
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

func (s *Store) ListDevices(ctx context.Context) ([]model.Device, error) {
	list := make([]model.Device, 0)
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, endpoints_json, ssh_host, target_profile_id, auth_type, auth_secret, key_path, ssh_user, ssh_port, remote_path, tags_json, enable_ping, enable_push, description, updated_at FROM devices ORDER BY name ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var d model.Device
		var epsJSON, tagsJSON string
		var ping, push int
		if err := rows.Scan(&d.ID, &d.Name, &epsJSON, &d.SSHHost, &d.TargetProfileID, &d.AuthType, &d.AuthSecret, &d.KeyPath, &d.SSHUser, &d.SSHPort, &d.RemotePath, &tagsJSON, &ping, &push, &d.Description, &d.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(epsJSON), &d.Endpoints)
		_ = json.Unmarshal([]byte(tagsJSON), &d.Tags)
		d.EnablePing = ping == 1
		d.EnablePush = push == 1
		list = append(list, d)
	}
	return list, rows.Err()
}

func (s *Store) SaveDevice(ctx context.Context, d model.Device) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	epsB, _ := json.Marshal(d.Endpoints)
	tagsB, _ := json.Marshal(d.Tags)
	ping := 0
	if d.EnablePing {
		ping = 1
	}
	push := 0
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
	_, err := s.db.ExecContext(ctx, query, d.ID, d.Name, string(epsB), d.SSHHost, d.TargetProfileID, d.AuthType, d.AuthSecret, d.KeyPath, d.SSHUser, d.SSHPort, d.RemotePath, string(tagsB), ping, push, d.Description, time.Now())
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

func quoteSlice(ids []string) string {
	quoted := make([]string, len(ids))
	for i, id := range ids {
		quoted[i] = fmt.Sprintf("'%s'", strings.ReplaceAll(id, "'", "''"))
	}
	return strings.Join(quoted, ",")
}
