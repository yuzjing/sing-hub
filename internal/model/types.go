// internal/model/types.go
package model

import "time"

type Strategy string

const (
	StrategyAppend Strategy = "append"
	StrategyMerge  Strategy = "merge"
)

type Component struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Category       string    `json:"category"`
	ScopeTag       string    `json:"scope_tag"`
	SingBoxVersion string    `json:"sing_box_version"`
	MountPath      string    `json:"mount_path"`
	Strategy       Strategy  `json:"strategy"`
	PayloadTmpl    string    `json:"payload_tmpl"`
	Description    string    `json:"description"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Module struct {
	ID          string     `json:"id"`
	TemplateID  string     `json:"template_id"`
	Name        string     `json:"name"`
	Category    string     `json:"category"`
	Tags        []string   `json:"tags"`
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
	Tags           []string               `json:"tags"`
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
	DeployStrategy  string       `json:"deploy_strategy"`
	ServiceName     string       `json:"service_name"`
	Tags            []string     `json:"tags"`
	EnablePing      bool         `json:"enable_ping"`
	EnablePush      bool         `json:"enable_push"`
	LastRemoteVer   string       `json:"last_remote_ver"` // 新增：最后一次成功探测到的节点远端版本
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

type UpdaterStatus struct {
	IsRunning           bool   `json:"is_running"`
	LocalStableVersion  string `json:"local_stable_version"`  // 本地物理存在的稳定版
	LocalAlphaVersion   string `json:"local_alpha_version"`   // 本地物理存在的尝鲜版
	RemoteStableVersion string `json:"remote_stable_version"` // GitHub 最新稳定版
	RemoteAlphaVersion  string `json:"remote_alpha_version"`  // GitHub 最新尝鲜版
	LastCheck           string `json:"last_check"`
	LastError           string `json:"last_error"`
}
