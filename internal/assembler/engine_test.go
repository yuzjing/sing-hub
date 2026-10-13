// internal/assembler/engine_test.go
package assembler_test

import (
	"context"
	"path/filepath"
	"testing"

	"sing-hub/internal/assembler"
	"sing-hub/internal/model"
	"sing-hub/internal/store"
)

func TestAssemblePipeline(t *testing.T) {
	ctx := context.Background()
	tmpDB := filepath.Join(t.TempDir(), "test.db")

	s, err := store.New(tmpDB)
	if err != nil {
		t.Fatalf("create store failed: %v", err)
	}
	defer s.Close()

	// 1. 设置一个全局变量
	err = s.SetVariable(ctx, model.Variable{
		Key:   "MY_UUID",
		Value: "11111111-2222-3333-4444-555555555555",
	})
	if err != nil {
		t.Fatalf("set variable failed: %v", err)
	}

	// 2. 插入两个组件：一个 Moonlight 规则 (route.rules)，一个 VPS 节点 (outbounds)
	c1 := model.Component{
		ID:        "moonlight-rule",
		Name:      "Moonlight Bypass",
		MountPath: "route.rules",
		Strategy:  model.StrategyAppend,
		PayloadJSON: `{
			"port": [47984, 47989],
			"outbound": "direct"
		}`,
	}
	c2 := model.Component{
		ID:        "vps-node",
		Name:      "VPS Singapore",
		MountPath: "outbounds",
		Strategy:  model.StrategyAppend,
		PayloadJSON: `{
			"type": "vless",
			"tag": "vps-out",
			"server": "argo.domain.com",
			"uuid": "{{ .MY_UUID }}"
		}`,
	}
	if err := s.SaveComponent(ctx, c1); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveComponent(ctx, c2); err != nil {
		t.Fatal(err)
	}

	// 3. 创建 Profile 并绑定这两个组件
	p := model.Profile{
		ID:     "tv-test",
		Name:   "Test TV",
		Target: model.TargetClient,
		Token:  "test-token",
		Components: []model.ProfileComponent{
			{ComponentID: "moonlight-rule", OrderIndex: 1},
			{ComponentID: "vps-node", OrderIndex: 2},
		},
	}
	if err := s.SaveProfile(ctx, p); err != nil {
		t.Fatal(err)
	}

	// 4. 执行编译
	eng := assembler.New(s)
	output, err := eng.Assemble(ctx, "tv-test")
	if err != nil {
		t.Fatalf("assemble failed: %v", err)
	}

	// 打印结果直观感受
	t.Logf("Compiled JSON:\n%s", string(output))
}
