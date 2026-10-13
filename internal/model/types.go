// internal/model/types.go

package model

import (
	"time"
)

type Strategy string

const (
	StrategyAppend Strategy = "append" // 追加到数组（如 inbounds, outbounds, route.rules）
	StrategyMerge  Strategy = "merge"  // 深度合并到对象（如 dns, log, experimental）
)

// TargetType 强类型枚举
type TargetType string

const (
	TargetClient TargetType = "client" // 客户端（TV、PC、手机等）
	TargetServer TargetType = "server" // 服务端（VPS、出口网关等）
)

// Variable 全局变量字典
type Variable struct {
	Key         string    `json:"key"`         // 占位符
	Value       string    `json:"value"`       // 值
	Description string    `json:"description"` // 描述
	UpdatedAt   time.Time `json:"updated_at"`  // 更新时间
}

// Component 原子资产
type Component struct {
	ID          string    `json:"id"`           // ID
	Name        string    `json:"name"`         // 名称
	Type        string    `json:"type"`         // 类型
	Category    string    `json:"category"`     // 分类
	MountPath   string    `json:"mount_path"`   // 挂载路径
	Strategy    Strategy  `json:"strategy"`     // 策略
	PayloadJSON string    `json:"payload_json"` // 原生局部JSON片段
	Description string    `json:"description"`  // 描述
	UpdatedAt   time.Time `json:"updated_at"`   // 更新时间
}

// ProfileComponent 设备装配关系（带有顺序权重）
type ProfileComponent struct {
	ComponentID string `json:"id"`          // ID
	OrderIndex  int    `json:"order_index"` // 顺序权重
}

// Profile 目标装配清单
type Profile struct {
	ID          string             `json:"id"`          // ID
	Name        string             `json:"name"`        // 名称
	Target      TargetType         `json:"target"`      // "client" 或 "server"
	Token       string             `json:"token"`       // 拉取订阅鉴权安全 Token
	Description string             `json:"description"` // 描述
	UpdatedAt   time.Time          `json:"updated_at"`  // 更新时间
	Components  []ProfileComponent `json:"components"`  // 组件列表
}
