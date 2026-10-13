// internal/store/sqlite.go
package store

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"sing-hub/internal/model"

	_ "modernc.org/sqlite" // 注册纯 Go sqlite 驱动
)

type Store struct {
	db *sql.DB
	mu sync.Mutex // 应用层写锁保护
}

// New 初始化 SQLite 连接池并自动迁移建表
func New(dbPath string) (*Store, error) {
	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite failed: %w", err)
	}

	db.SetMaxOpenConns(1) // 单文件数据库写连接设为 1 最安全稳定

	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("auto migration failed: %w", err)
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// migrate 自动初始化数据库表结构
func (s *Store) migrate(ctx context.Context) error {
	ddl := `
	-- 1. 全局变量表
	CREATE TABLE IF NOT EXISTS variables (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		updated_at DATETIME NOT NULL
	);

	-- 2. 通用原子组件资产表
	CREATE TABLE IF NOT EXISTS components (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		category TEXT NOT NULL,
		mount_path TEXT NOT NULL,
		strategy TEXT NOT NULL,
		payload_json TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		updated_at DATETIME NOT NULL
	);

	-- 3. 目标设备档案表
	CREATE TABLE IF NOT EXISTS profiles (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		target TEXT NOT NULL,
		token TEXT NOT NULL UNIQUE,
		description TEXT NOT NULL DEFAULT '',
		updated_at DATETIME NOT NULL
	);

	-- 4. 设备与组件的装配关系表 (按 order_index 升序排序)
	CREATE TABLE IF NOT EXISTS profile_components (
		profile_id TEXT NOT NULL,
		component_id TEXT NOT NULL,
		order_index INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (profile_id, component_id),
		FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE,
		FOREIGN KEY (component_id) REFERENCES components(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_profile_order ON profile_components(profile_id, order_index ASC);
	`
	_, err := s.db.ExecContext(ctx, ddl)
	return err
}

// ==================== 全局变量 (Variables) ====================

func (s *Store) ListVariables(ctx context.Context) ([]model.Variable, error) {
	list := make([]model.Variable, 0) // 初始化为空切片，确保序列化为 []
	rows, err := s.db.QueryContext(ctx, "SELECT key, value, description, updated_at FROM variables ORDER BY key ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var v model.Variable
		if err := rows.Scan(&v.Key, &v.Value, &v.Description, &v.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, v)
	}
	return list, rows.Err()
}

func (s *Store) SetVariable(ctx context.Context, v model.Variable) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO variables (key, value, description, updated_at)
	VALUES (?, ?, ?, ?)
	ON CONFLICT(key) DO UPDATE SET
		value = excluded.value,
		description = excluded.description,
		updated_at = excluded.updated_at;
	`
	_, err := s.db.ExecContext(ctx, query, v.Key, v.Value, v.Description, time.Now())
	return err
}

func (s *Store) DeleteVariable(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, "DELETE FROM variables WHERE key = ?", key)
	return err
}

// ==================== 组件资产 (Components) ====================

func (s *Store) ListComponents(ctx context.Context) ([]model.Component, error) {
	query := "SELECT id, name, category, mount_path, strategy, payload_json, description, updated_at FROM components ORDER BY category ASC, name ASC"
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]model.Component, 0)
	for rows.Next() {
		var c model.Component
		if err := rows.Scan(&c.ID, &c.Name, &c.Category, &c.MountPath, &c.Strategy, &c.PayloadJSON, &c.Description, &c.UpdatedAt); err != nil {
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
	INSERT INTO components (id, name, category, mount_path, strategy, payload_json, description, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		name = excluded.name,
		category = excluded.category,
		mount_path = excluded.mount_path,
		strategy = excluded.strategy,
		payload_json = excluded.payload_json,
		description = excluded.description,
		updated_at = excluded.updated_at;
	`
	_, err := s.db.ExecContext(ctx, query, c.ID, c.Name, c.Category, c.MountPath, c.Strategy, c.PayloadJSON, c.Description, time.Now())
	return err
}

func (s *Store) DeleteComponent(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, "DELETE FROM components WHERE id = ?", id)
	return err
}

// ==================== 设备档案与装配关系 (Profiles) ====================

func (s *Store) ListProfiles(ctx context.Context) ([]model.Profile, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, target, token, description, updated_at FROM profiles ORDER BY name ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]model.Profile, 0)
	for rows.Next() {
		var p model.Profile
		if err := rows.Scan(&p.ID, &p.Name, &p.Target, &p.Token, &p.Description, &p.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (s *Store) GetProfile(ctx context.Context, id string) (*model.Profile, error) {
	var p model.Profile
	row := s.db.QueryRowContext(ctx, "SELECT id, name, target, token, description, updated_at FROM profiles WHERE id = ?", id)
	if err := row.Scan(&p.ID, &p.Name, &p.Target, &p.Token, &p.Description, &p.UpdatedAt); err != nil {
		return nil, err
	}

	// 查出该 Profile 绑定的组件 ID 与顺序
	bindRows, err := s.db.QueryContext(ctx, "SELECT component_id, order_index FROM profile_components WHERE profile_id = ? ORDER BY order_index ASC", id)
	if err != nil {
		return nil, err
	}
	defer bindRows.Close()

	for bindRows.Next() {
		var b model.ProfileComponent
		if err := bindRows.Scan(&b.ComponentID, &b.OrderIndex); err != nil {
			return nil, err
		}
		p.Components = append(p.Components, b)
	}
	if err := bindRows.Err(); err != nil {
		return nil, err
	}
	return &p, nil
}

// GetProfileByToken 供客户端匿名安全拉取配置的查询入口
func (s *Store) GetProfileByToken(ctx context.Context, token string) (*model.Profile, error) {
	var p model.Profile
	row := s.db.QueryRowContext(ctx, "SELECT id, name, target, token, description, updated_at FROM profiles WHERE token = ?", token)
	if err := row.Scan(&p.ID, &p.Name, &p.Target, &p.Token, &p.Description, &p.UpdatedAt); err != nil {
		return nil, err
	}
	return s.GetProfile(ctx, p.ID)
}

// SaveProfile 事务保存：同时更新 Profile 元数据并原子替换其绑定的组件清单
func (s *Store) SaveProfile(ctx context.Context, p model.Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. 保存 Profile 元数据
	pQuery := `
	INSERT INTO profiles (id, name, target, token, description, updated_at)
	VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		name = excluded.name,
		target = excluded.target,
		token = excluded.token,
		description = excluded.description,
		updated_at = excluded.updated_at;
	`
	if _, err := tx.ExecContext(ctx, pQuery, p.ID, p.Name, p.Target, p.Token, p.Description, time.Now()); err != nil {
		return err
	}

	// 2. 清理旧绑定并重新插入新绑定
	if _, err := tx.ExecContext(ctx, "DELETE FROM profile_components WHERE profile_id = ?", p.ID); err != nil {
		return err
	}

	bStmt, err := tx.PrepareContext(ctx, "INSERT INTO profile_components (profile_id, component_id, order_index) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer bStmt.Close()

	for _, c := range p.Components {
		if _, err := bStmt.ExecContext(ctx, p.ID, c.ComponentID, c.OrderIndex); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) DeleteProfile(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, "DELETE FROM profiles WHERE id = ?", id)
	return err
}

// GetResolvedProfileComponents 拼装引擎专用：一次性按 order_index ASC 查出全部实体！
func (s *Store) GetResolvedProfileComponents(ctx context.Context, profileID string) ([]model.Component, error) {
	query := `
	SELECT c.id, c.name, c.category, c.mount_path, c.strategy, c.payload_json, c.description, c.updated_at
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

	var list []model.Component
	for rows.Next() {
		var c model.Component
		if err := rows.Scan(&c.ID, &c.Name, &c.Category, &c.MountPath, &c.Strategy, &c.PayloadJSON, &c.Description, &c.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}
