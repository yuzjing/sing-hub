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

// PreviewComponent 工坊沙箱预览：直接挂钩真实节点真实数据，确保 100% 渲染出合法 JSON
func (e *Engine) PreviewComponent(ctx context.Context, tmplStr string, paramsJSON string, nodeID string) (string, error) {
	var params map[string]any
	if strings.TrimSpace(paramsJSON) == "" {
		params = make(map[string]any)
	} else {
		if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
			params = make(map[string]any)
		}
	}

	// 1. 注入真实节点的物理数据
	var nodeCtx map[string]any
	if nodeID != "" {
		devices, _ := e.store.ListDevices(ctx)
		for _, d := range devices {
			if d.ID == nodeID {
				primaryEP := ""
				if len(d.Endpoints) > 0 {
					primaryEP = d.Endpoints[0]
				}
				nodeCtx = map[string]any{
					"id":       d.ID,
					"name":     d.Name,
					"endpoint": primaryEP,
					"ssh_host": d.SSHHost,
					"tags":     d.Tags,
				}
				break
			}
		}
	}

	if nodeCtx == nil {
		nodeCtx = map[string]any{
			"id":       "node-preview",
			"name":     "测试节点",
			"endpoint": "127.0.0.1",
			"ssh_host": "127.0.0.1",
			"tags":     []string{"preview"},
		}
	}
	params["_node"] = nodeCtx

	if _, exists := params["_profile"]; !exists {
		params["_profile"] = map[string]any{
			"id":      "preview-profile",
			"name":    "预览测试方案",
			"token":   "tok_test_sample",
			"version": "1.11.0",
		}
	}

	tmpl, err := template.New("preview").Funcs(funcMap()).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("template parse error: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("template execute error: %w", err)
	}

	var validate any
	if err := json.Unmarshal(buf.Bytes(), &validate); err != nil {
		return "", fmt.Errorf("rendered invalid json: %w\n%s", err, buf.String())
	}

	pretty, _ := json.MarshalIndent(validate, "", "  ")
	return string(pretty), nil
}

// Assemble 编译方案
func (e *Engine) Assemble(ctx context.Context, profileID string) ([]byte, string, error) {
	start := time.Now()
	slog.Info("compiling profile", "profile_id", profileID)

	profile, err := e.store.GetProfile(ctx, profileID)
	if err != nil {
		return nil, "", fmt.Errorf("get profile failed: %w", err)
	}

	bindings, err := e.store.GetResolvedProfileBindings(ctx, profileID)
	if err != nil {
		return nil, "", fmt.Errorf("load bindings failed: %w", err)
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

	for _, b := range bindings {
		var params map[string]any
		if strings.TrimSpace(b.ParamsJSON) == "" {
			params = make(map[string]any)
		} else {
			_ = json.Unmarshal([]byte(b.ParamsJSON), &params)
		}

		params["_profile"] = map[string]any{
			"id":      profile.ID,
			"name":    profile.Name,
			"token":   profile.Token,
			"version": profile.SingBoxVersion,
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
				"name":     "未关联物理节点",
				"endpoint": "127.0.0.1",
				"ssh_host": "127.0.0.1",
				"tags":     []string{},
			}
		}

		tmpl, err := template.New(b.Component.ID).Funcs(funcMap()).Parse(b.Component.PayloadTmpl)
		if err != nil {
			return nil, "", fmt.Errorf("component [%s] template syntax error: %w", b.Component.Name, err)
		}

		var rendered bytes.Buffer
		if err := tmpl.Execute(&rendered, params); err != nil {
			return nil, "", fmt.Errorf("component [%s] render error: %w", b.Component.Name, err)
		}

		var chunk any
		if err := json.Unmarshal(rendered.Bytes(), &chunk); err != nil {
			return nil, "", fmt.Errorf("component [%s] rendered invalid json: %w\n%s", b.Component.Name, err, rendered.String())
		}

		if err := mount(root, b.Component.MountPath, b.Component.Strategy, chunk); err != nil {
			return nil, "", fmt.Errorf("mount component [%s] failed: %w", b.Component.Name, err)
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

	slog.Info("profile compilation finished",
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
