// internal/model/types.go
package model

import "time"

type Strategy string

const (
	StrategyAppend Strategy = "append"
	StrategyMerge  Strategy = "merge"
)

type Component struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Category    string    `json:"category"`     // 'io' | 'routing' | 'dns' | 'system'
	ScopeTag    string    `json:"scope_tag"`    // 逗号分割标签
	MountPath   string    `json:"mount_path"`   // 挂载点
	Strategy    Strategy  `json:"strategy"`     // "append" | "merge"
	PayloadTmpl string    `json:"payload_tmpl"` // Go 原生模板源码
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ProfileComponentBinding struct {
	ComponentID string `json:"component_id"`
	OrderIndex  int    `json:"order_index"`
	ParamsJSON  string `json:"params_json"`
}

type ProfileCapabilities struct {
	EnableSubscribe bool `json:"enable_subscribe"`
	EnablePing      bool `json:"enable_ping"`
	EnablePush      bool `json:"enable_push"`
}

type Profile struct {
	ID             string                    `json:"id"`
	Name           string                    `json:"name"`
	Token          string                    `json:"token"`
	Description    string                    `json:"description"`
	SingBoxVersion string                    `json:"sing_box_version"` // 适配的目标内核版本
	Capabilities   ProfileCapabilities       `json:"capabilities"`
	Components     []ProfileComponentBinding `json:"components"`
	UpdatedAt      time.Time                 `json:"updated_at"`
}

type Device struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Endpoints       []string  `json:"endpoints"` // 流量通信端点列表 (IPv4, IPv6, 域名)
	SSHHost         string    `json:"ssh_host"`  // 专属 SSH 运维目标 (可独立指定)
	TargetProfileID string    `json:"target_profile_id"`
	AuthType        string    `json:"auth_type"` // 'key_path' | 'key_content' | 'password'
	AuthSecret      string    `json:"auth_secret"`
	KeyPath         string    `json:"key_path"`
	SSHUser         string    `json:"ssh_user"`
	SSHPort         int       `json:"ssh_port"`
	RemotePath      string    `json:"remote_path"`
	Tags            []string  `json:"tags"`
	EnablePing      bool      `json:"enable_ping"`
	EnablePush      bool      `json:"enable_push"`
	Description     string    `json:"description"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type CommitRecord struct {
	Hash      string    `json:"hash"`
	Message   string    `json:"message"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
}

type BatchDeleteReq struct {
	IDs []string `json:"ids"`
}
