// internal/model/types.go
package model

import "time"

type Strategy string

const (
	StrategyAppend Strategy = "append"
	StrategyMerge  Strategy = "merge"
)

// Component 第 1 层：原子工坊官方字段母版 (纯语法模板)
type Component struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Category    string    `json:"category"`     // 'io' | 'routing' | 'dns' | 'system'
	ScopeTag    string    `json:"scope_tag"`    // 逗号分割标签
	MountPath   string    `json:"mount_path"`   // 挂载点，如 "outbounds", "dns.servers"
	Strategy    Strategy  `json:"strategy"`     // "append" | "merge"
	PayloadTmpl string    `json:"payload_tmpl"` // Go 官方模板源码
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Module 第 2 层：成品积木 (基于母版填入实参，形成独立实体)
type Module struct {
	ID          string    `json:"id"`
	TemplateID  string    `json:"template_id"` // 指向 Component.ID (软链接指针)
	Name        string    `json:"name"`        // 成品名称，如 "香港 CN2 出站"
	Category    string    `json:"category"`    // 继承或自定义分类
	ParamsJSON  string    `json:"params_json"` // 该成品的具体实参 JSON
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`

	// 关联字段 (只读关联，供前端直接展示挂载点)
	Template *Component `json:"template,omitempty"`
}

// ProfileModuleBinding 第 3 层方案所挂载的成品模块引用
type ProfileModuleBinding struct {
	ID         string `json:"id"` // 绑定实例唯一键 (支持多实例)
	ProfileID  string `json:"profile_id"`
	ModuleID   string `json:"module_id"`
	OrderIndex int    `json:"order_index"`
}

type ProfileCapabilities struct {
	EnableSubscribe bool `json:"enable_subscribe"`
	EnablePing      bool `json:"enable_ping"`
	EnablePush      bool `json:"enable_push"`
}

// Profile 第 3 层：方案装配配方
type Profile struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Token          string                 `json:"token"`
	Description    string                 `json:"description"`
	SingBoxVersion string                 `json:"sing_box_version"`
	Capabilities   ProfileCapabilities    `json:"capabilities"`
	Modules        []ProfileModuleBinding `json:"modules"` // 挂载的成品积木列表
	UpdatedAt      time.Time              `json:"updated_at"`
}

type Device struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Endpoints       []string  `json:"endpoints"`
	SSHHost         string    `json:"ssh_host"`
	TargetProfileID string    `json:"target_profile_id"`
	AuthType        string    `json:"auth_type"`
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
