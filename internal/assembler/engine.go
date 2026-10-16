// internal/assembler/engine.go
package assembler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"text/template"
	"time"

	"sing-hub/internal/model"
	"sing-hub/internal/store"
	"sing-hub/internal/versioning"
)

type Engine struct {
	store   *store.Store
	gitRepo *versioning.GitRepo
}

func New(s *store.Store, g *versioning.GitRepo) *Engine {
	return &Engine{store: s, gitRepo: g}
}

func funcMap() template.FuncMap {
	return template.FuncMap{
		"json": func(v any) (string, error) {
			b, err := json.Marshal(v)
			return string(b), err
		},
		"default": func(def any, val any) any {
			if val == nil || val == "" {
				return def
			}
			return val
		},
	}
}

// unflattenParams 将类似 "_node.endpoint" 扁平键转换为嵌套结构
func unflattenParams(raw map[string]any) map[string]any {
	result := make(map[string]any)
	for key, val := range raw {
		cleanKey := strings.TrimPrefix(key, ".")
		parts := strings.Split(cleanKey, ".")
		curr := result
		for i := 0; i < len(parts)-1; i++ {
			p := parts[i]
			if _, exists := curr[p]; !exists {
				curr[p] = make(map[string]any)
			}
			if nextMap, ok := curr[p].(map[string]any); ok {
				curr = nextMap
			}
		}
		curr[parts[len(parts)-1]] = val
	}
	return result
}

// resolveNodeReferences 解析 @node:<node_id>.<prop> 动态引用
// resolveNodeReferences 增强版：支持 @node:<id>.ipv4, @node:<id>.ipv6, @node:<id>.domain, @node:<id>.endpoint
func (e *Engine) resolveNodeReferences(ctx context.Context, params map[string]any) {
	devices, err := e.store.ListDevices(ctx)
	if err != nil {
		return
	}
	devMap := make(map[string]model.Device)
	for _, d := range devices {
		devMap[d.ID] = d
	}

	isIPv4 := func(ip string) bool {
		return strings.Count(ip, ".") == 3 && !strings.Contains(ip, ":")
	}
	isIPv6 := func(ip string) bool {
		return strings.Contains(ip, ":")
	}

	var walk func(m map[string]any)
	walk = func(m map[string]any) {
		for k, v := range m {
			if strVal, ok := v.(string); ok {
				if strings.HasPrefix(strVal, "@node:") {
					parts := strings.Split(strings.TrimPrefix(strVal, "@node:"), ".")
					if len(parts) >= 2 {
						nodeID := parts[0]
						prop := parts[1]
						if dev, exists := devMap[nodeID]; exists {
							switch prop {
							case "endpoint":
								if len(dev.Endpoints) > 0 {
									m[k] = dev.Endpoints[0]
								} else {
									m[k] = "127.0.0.1"
								}
							case "ipv4":
								found := ""
								for _, ep := range dev.Endpoints {
									if isIPv4(ep) {
										found = ep
										break
									}
								}
								if found == "" && len(dev.Endpoints) > 0 {
									found = dev.Endpoints[0]
								}
								m[k] = found
							case "ipv6":
								found := ""
								for _, ep := range dev.Endpoints {
									if isIPv6(ep) {
										found = ep
										break
									}
								}
								if found == "" && len(dev.Endpoints) > 0 {
									found = dev.Endpoints[0]
								}
								m[k] = found
							case "domain":
								found := ""
								for _, ep := range dev.Endpoints {
									if !isIPv4(ep) && !isIPv6(ep) {
										found = ep
										break
									}
								}
								if found == "" && len(dev.Endpoints) > 0 {
									found = dev.Endpoints[0]
								}
								m[k] = found
							case "ssh_host":
								m[k] = dev.SSHHost
							case "id":
								m[k] = dev.ID
							case "name":
								m[k] = dev.Name
							}
						}
					}
				} else if strings.HasPrefix(strVal, "@sec:token") {
					m[k] = e.store.GetSetting(ctx, "git_token", "")
				} else if strings.HasPrefix(strVal, "@net:easytier_cidr") {
					m[k] = e.store.GetSetting(ctx, "easytier_cidr", "10.144.0.0/16")
				}
			} else if subMap, ok := v.(map[string]any); ok {
				walk(subMap)
			}
		}
	}
	walk(params)
}

func (e *Engine) PreviewComponent(ctx context.Context, tmplStr string, paramsJSON string) (string, error) {
	if strings.TrimSpace(tmplStr) == "" {
		return "{\n}", nil
	}

	rawParams := make(map[string]any)
	if strings.TrimSpace(paramsJSON) != "" {
		_ = json.Unmarshal([]byte(paramsJSON), &rawParams)
	}

	params := unflattenParams(rawParams)
	e.resolveNodeReferences(ctx, params)

	// 注入默认上下文，防止留空时模板崩溃
	nodeMap, ok := params["_node"].(map[string]any)
	if !ok {
		nodeMap = make(map[string]any)
		params["_node"] = nodeMap
	}
	if _, ok := nodeMap["id"]; !ok {
		nodeMap["id"] = "mock-node"
	}
	if _, ok := nodeMap["name"]; !ok {
		nodeMap["name"] = "测试节点"
	}
	if _, ok := nodeMap["endpoint"]; !ok {
		nodeMap["endpoint"] = "10.123.123.1"
	}
	if _, ok := nodeMap["ssh_host"]; !ok {
		nodeMap["ssh_host"] = "10.123.123.1"
	}

	if _, ok := params["_net"]; !ok {
		params["_net"] = map[string]any{"easytier_cidr": "10.144.0.0/16"}
	}
	if _, ok := params["_sec"]; !ok {
		params["_sec"] = map[string]any{"uuid": "a1b2c3d4-e5f6-7890-abcd-ef1234567890", "password": "mock_password"}
	}
	if _, ok := params["_profile"]; !ok {
		params["_profile"] = map[string]any{"id": "mock-profile", "name": "测试方案", "token": "mock_token", "version": "1.10.0"}
	}

	tmpl, err := template.New("preview").Funcs(funcMap()).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("模板解析错误: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("模板执行错误: %w", err)
	}

	var validate any
	if err := json.Unmarshal(buf.Bytes(), &validate); err != nil {
		return "", fmt.Errorf("输出不是合法 JSON: %w\n渲染结果:\n%s", err, buf.String())
	}

	pretty, _ := json.MarshalIndent(validate, "", "  ")
	return string(pretty), nil
}

func (e *Engine) Assemble(ctx context.Context, profileID string) ([]byte, string, error) {
	start := time.Now()
	slog.Info("assembling profile from modules", "profile_id", profileID)

	profile, err := e.store.GetProfile(ctx, profileID)
	if err != nil {
		return nil, "", fmt.Errorf("get profile failed: %w", err)
	}

	resolvedModules, err := e.store.GetResolvedProfileModules(ctx, profileID)
	if err != nil {
		return nil, "", fmt.Errorf("load resolved modules failed: %w", err)
	}

	var associatedDevice *model.Device
	devices, _ := e.store.ListDevices(ctx)
	for _, d := range devices {
		if d.TargetProfileID == profileID {
			associatedDevice = &d
			break
		}
	}

	root := make(map[string]any)

	for _, item := range resolvedModules {
		rawParams := make(map[string]any)
		if strings.TrimSpace(item.Module.ParamsJSON) != "" {
			_ = json.Unmarshal([]byte(item.Module.ParamsJSON), &rawParams)
		}

		params := unflattenParams(rawParams)
		e.resolveNodeReferences(ctx, params)

		params["_profile"] = map[string]any{
			"id":      profile.ID,
			"name":    profile.Name,
			"token":   profile.Token,
			"version": profile.SingBoxVersion,
		}

		params["_net"] = map[string]any{
			"easytier_cidr": e.store.GetSetting(ctx, "easytier_cidr", "10.144.0.0/16"),
		}

		params["_sec"] = map[string]any{
			"uuid":     profile.Token,
			"password": profile.Token,
		}

		if associatedDevice != nil {
			primaryEP := ""
			if len(associatedDevice.Endpoints) > 0 {
				primaryEP = associatedDevice.Endpoints[0]
			}
			params["_node"] = map[string]any{
				"id":       associatedDevice.ID,
				"name":     associatedDevice.Name,
				"endpoint": primaryEP,
				"ssh_host": associatedDevice.SSHHost,
				"tags":     associatedDevice.Tags,
			}
		} else {
			params["_node"] = map[string]any{
				"id":       "unassigned",
				"name":     "未关联节点",
				"endpoint": "127.0.0.1",
				"ssh_host": "127.0.0.1",
				"tags":     []string{},
			}
		}

		tmpl, err := template.New(item.Module.ID).Funcs(funcMap()).Parse(item.Template.PayloadTmpl)
		if err != nil {
			return nil, "", fmt.Errorf("积木 [%s] 语法错误: %w", item.Module.Name, err)
		}

		var rendered bytes.Buffer
		if err := tmpl.Execute(&rendered, params); err != nil {
			return nil, "", fmt.Errorf("积木 [%s] 渲染错误: %w", item.Module.Name, err)
		}

		var chunk any
		if err := json.Unmarshal(rendered.Bytes(), &chunk); err != nil {
			return nil, "", fmt.Errorf("积木 [%s] 产生非法 JSON: %w\n%s", item.Module.Name, err, rendered.String())
		}

		if err := mount(root, item.Template.MountPath, item.Template.Strategy, chunk); err != nil {
			return nil, "", fmt.Errorf("装配积木 [%s] 失败: %w", item.Module.Name, err)
		}
	}

	finalJSON, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, "", err
	}

	var commitHash string
	if e.gitRepo != nil {
		sqlDump, _ := e.store.DumpSQL(ctx)
		commitHash, _ = e.gitRepo.CommitSnapshot(profileID, finalJSON, sqlDump)
	}

	slog.Info("profile assembly finished",
		"profile_id", profileID,
		"duration_ms", time.Since(start).Milliseconds(),
		"size_bytes", len(finalJSON),
	)

	return finalJSON, commitHash, nil
}

func mount(root map[string]any, mountPath string, strategy model.Strategy, chunk any) error {
	mountPath = strings.TrimSpace(mountPath)
	if mountPath == "" {
		if strategy == model.StrategyMerge {
			if m, ok := chunk.(map[string]any); ok {
				deepMerge(root, m)
				return nil
			}
		}
		return fmt.Errorf("empty mount path only supports object merge")
	}

	parts := strings.Split(mountPath, ".")
	curr := root

	for i := 0; i < len(parts)-1; i++ {
		key := parts[i]
		next, exists := curr[key]
		if !exists {
			nextMap := make(map[string]any)
			curr[key] = nextMap
			curr = nextMap
		} else {
			nextMap, ok := next.(map[string]any)
			if !ok {
				return fmt.Errorf("path conflict: %s is not an object", key)
			}
			curr = nextMap
		}
	}

	leafKey := parts[len(parts)-1]

	switch strategy {
	case model.StrategyAppend:
		var targetSlice []any
		if existing, exists := curr[leafKey]; exists {
			s, ok := existing.([]any)
			if !ok {
				return fmt.Errorf("mount conflict: %s is not an array for append", leafKey)
			}
			targetSlice = s
		} else {
			targetSlice = make([]any, 0)
		}

		if chunkSlice, ok := chunk.([]any); ok {
			targetSlice = append(targetSlice, chunkSlice...)
		} else {
			targetSlice = append(targetSlice, chunk)
		}
		curr[leafKey] = targetSlice

	case model.StrategyMerge:
		var targetMap map[string]any
		if existing, exists := curr[leafKey]; exists {
			m, ok := existing.(map[string]any)
			if !ok {
				return fmt.Errorf("mount conflict: %s is not an object for merge", leafKey)
			}
			targetMap = m
		} else {
			targetMap = make(map[string]any)
			curr[leafKey] = targetMap
		}

		chunkMap, ok := chunk.(map[string]any)
		if !ok {
			return fmt.Errorf("merge payload must be an object")
		}
		deepMerge(targetMap, chunkMap)

	default:
		return fmt.Errorf("unsupported strategy: %s", strategy)
	}

	return nil
}

func deepMerge(dst, src map[string]any) {
	for k, v := range src {
		if srcMap, ok := v.(map[string]any); ok {
			if dstMap, exists := dst[k].(map[string]any); exists {
				deepMerge(dstMap, srcMap)
				continue
			}
		}
		dst[k] = v
	}
}
