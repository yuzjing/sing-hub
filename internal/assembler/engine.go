// internal/assembler/engine.go
package assembler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"sing-hub/internal/model"
	"sing-hub/internal/store"
)

type Engine struct {
	store *store.Store
}

func New(s *store.Store) *Engine {
	return &Engine{store: s}
}

// Assemble 将指定 Profile 编译为完整的 sing-box JSON 配置文件
func (e *Engine) Assemble(ctx context.Context, profileID string) ([]byte, error) {
	// 1. 读取变量字典
	vars, err := e.store.ListVariables(ctx)
	if err != nil {
		return nil, fmt.Errorf("load variables: %w", err)
	}
	varMap := make(map[string]string, len(vars))
	for _, v := range vars {
		varMap[v.Key] = v.Value
	}

	// 2. 查出当前 Profile 绑定的组件 (SQL 已按 order_index ASC 排好序)
	components, err := e.store.GetResolvedProfileComponents(ctx, profileID)
	if err != nil {
		return nil, fmt.Errorf("load profile components: %w", err)
	}

	// 3. 初始化空根节点
	root := make(map[string]any)

	// 4. 逐个装配组件
	for _, comp := range components {
		// 变量插值替换
		resolvedPayload := interpolate(comp.PayloadJSON, varMap)

		// 弱类型解析为原生 any 数据 (可能是 map 或 slice)
		var chunk any
		if err := json.Unmarshal([]byte(resolvedPayload), &chunk); err != nil {
			return nil, fmt.Errorf("component [%s] invalid json: %w", comp.Name, err)
		}

		// 沿着 MountPath 挂载
		if err := mount(root, comp.MountPath, comp.Strategy, chunk); err != nil {
			return nil, fmt.Errorf("mount component [%s] to path [%s]: %w", comp.Name, comp.MountPath, err)
		}
	}

	// 5. 格式化输出
	return json.MarshalIndent(root, "", "  ")
}

// interpolate 极简高效的变量占位符替换，如 {{ .MY_UUID }}
func interpolate(template string, vars map[string]string) string {
	res := template
	for k, v := range vars {
		placeholder := fmt.Sprintf("{{ .%s }}", k)
		res = strings.ReplaceAll(res, placeholder, v)
		// 容错: 同时支持不带空格的形式 {{.MY_UUID}}
		placeholderNoSpace := fmt.Sprintf("{{.%s}}", k)
		res = strings.ReplaceAll(res, placeholderNoSpace, v)
	}
	return res
}

// mount 沿点分路径导航，并将 chunk 按照策略合并入 root
func mount(root map[string]any, mountPath string, strategy model.Strategy, chunk any) error {
	mountPath = strings.TrimSpace(mountPath)
	if mountPath == "" {
		// 顶级补丁: 若路径为空且为 merge，直接把 chunk 根合并进 root
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

	// 沿途导航至倒数第二级字典
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
		// 目标必须为切片 []any
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

		// 感知展平 (Concat vs Push)
		if chunkSlice, ok := chunk.([]any); ok {
			targetSlice = append(targetSlice, chunkSlice...)
		} else {
			targetSlice = append(targetSlice, chunk)
		}
		curr[leafKey] = targetSlice

	case model.StrategyMerge:
		// 目标必须为字典 map[string]any
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

// deepMerge 递归合并字典，src 覆盖 dst
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
