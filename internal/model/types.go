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
	Category    string    `json:"category"`
	ScopeTag    string    `json:"scope_tag"`
	MountPath   string    `json:"mount_path"`
	Strategy    Strategy  `json:"strategy"`
	PayloadTmpl string    `json:"payload_tmpl"`
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Module struct {
	ID          string     `json:"id"`
	TemplateID  string     `json:"template_id"`
	Name        string     `json:"name"`
	Category    string     `json:"category"`
	Tags        []string   `json:"tags"` // 补齐积木标签
	ParamsJSON  string     `json:"params_json"`
	Description string     `json:"description"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Template    *Component `json:"template,omitempty"`
}

type ProfileModuleBinding struct {
	ID         string `json:"id"`
	ProfileID  string `json:"profile_id"`
	ModuleID   string `json:"module_id"`
	OrderIndex int    `json:"order_index"`
}

type ProfileCapabilities struct {
	EnableSubscribe bool `json:"enable_subscribe"`
	EnablePing      bool `json:"enable_ping"`
	EnablePush      bool `json:"enable_push"`
}

type Profile struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Token          string                 `json:"token"`
	Tags           []string               `json:"tags"` // 补齐方案标签
	Description    string                 `json:"description"`
	SingBoxVersion string                 `json:"sing_box_version"`
	Capabilities   ProfileCapabilities    `json:"capabilities"`
	Modules        []ProfileModuleBinding `json:"modules"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

type DeviceProp struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value string `json:"value"`
}

type Device struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Endpoints       []string     `json:"endpoints"`
	Props           []DeviceProp `json:"props"`
	SSHHost         string       `json:"ssh_host"`
	TargetProfileID string       `json:"target_profile_id"`
	AuthType        string       `json:"auth_type"`
	AuthSecret      string       `json:"auth_secret"`
	KeyPath         string       `json:"key_path"`
	SSHUser         string       `json:"ssh_user"`
	SSHPort         int          `json:"ssh_port"`
	RemotePath      string       `json:"remote_path"`
	Tags            []string     `json:"tags"`
	EnablePing      bool         `json:"enable_ping"`
	EnablePush      bool         `json:"enable_push"`
	Description     string       `json:"description"`
	UpdatedAt       time.Time    `json:"updated_at"`
}

type CommitRecord struct {
	Hash      string    `json:"hash"`
	Message   string    `json:"message"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
}

type NodePingReport struct {
	DeviceID  string `json:"device_id"`
	Endpoint  string `json:"endpoint"`
	Online    bool   `json:"online"`
	LatencyMS int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

type BatchDeleteReq struct {
	IDs []string `json:"ids"`
}
