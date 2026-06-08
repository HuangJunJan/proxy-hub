package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/tidwall/gjson"

	"github.com/huangjunjan/proxy-hub/internal/config"
	"github.com/huangjunjan/proxy-hub/internal/store"
)

func TestValidate(t *testing.T) {
	if err := Validate(map[string]any{"type": "stdio", "command": "npx"}); err != nil {
		t.Errorf("合法 stdio 不应报错: %v", err)
	}
	if err := Validate(map[string]any{"command": "npx"}); err != nil {
		t.Errorf("缺省 type=stdio 应合法: %v", err)
	}
	if err := Validate(map[string]any{"type": "stdio"}); err == nil {
		t.Error("stdio 缺 command 应报错")
	}
	if err := Validate(map[string]any{"type": "http"}); err == nil {
		t.Error("http 缺 url 应报错")
	}
	if err := Validate(map[string]any{"type": "http", "url": "https://x"}); err != nil {
		t.Errorf("合法 http 不应报错: %v", err)
	}
	if err := Validate(map[string]any{"type": "weird", "command": "x"}); err == nil {
		t.Error("未知 type 应报错")
	}
}

func TestApplyClaudePreservesOtherKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".claude.json")
	original := `{
  "numStartups": 5,
  "projects": {"/home/x": {"history": [1, 2, 3]}},
  "mcpServers": {"unmanaged": {"command": "foo"}}
}`
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	servers := []Server{{ID: "ctx7", Spec: map[string]any{"type": "stdio", "command": "npx", "args": []any{"-y", "ctx7"}}, EnabledClaude: true}}
	if err := ApplyClaude(path, servers, false); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	g := gjson.ParseBytes(raw)
	if g.Get("numStartups").Int() != 5 {
		t.Error("numStartups 应保留")
	}
	if g.Get(`projects./home/x.history.0`).Int() != 1 {
		t.Error("projects 历史应保留")
	}
	if g.Get("mcpServers.unmanaged.command").String() != "foo" {
		t.Error("未管理的 mcpServers 条目应保留")
	}
	if g.Get("mcpServers.ctx7.command").String() != "npx" {
		t.Error("新 server 应写入")
	}
	// .bak 应在首次写后存在。
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Errorf("首次写后应有 .bak: %v", err)
	}
}

func TestApplyClaudeRemovesDisabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".claude.json")
	os.WriteFile(path, []byte(`{"mcpServers":{"ctx7":{"command":"npx"}}}`), 0o600)
	servers := []Server{{ID: "ctx7", Spec: map[string]any{"type": "stdio", "command": "npx"}, EnabledClaude: false}}
	if err := ApplyClaude(path, servers, false); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if gjson.GetBytes(raw, "mcpServers.ctx7").Exists() {
		t.Error("禁用的 server 应被移除")
	}
}

func TestClaudeCmdCWrap(t *testing.T) {
	obj := claudeObject(Server{Spec: map[string]any{"type": "stdio", "command": "npx", "args": []any{"-y", "x"}}}, true)
	if obj["command"] != "cmd" {
		t.Errorf("command 应被包装为 cmd，实际 %v", obj["command"])
	}
	args, _ := obj["args"].([]any)
	if len(args) != 4 || args[0] != "/c" || args[1] != "npx" {
		t.Errorf("args 应为 [/c npx -y x]，实际 %v", args)
	}
	// 非 wrappable 命令不包装。
	obj2 := claudeObject(Server{Spec: map[string]any{"type": "stdio", "command": "/usr/local/bin/my-mcp"}}, true)
	if obj2["command"] != "/usr/local/bin/my-mcp" {
		t.Error("非 npx 类命令不应包装")
	}
}

func TestApplyCodexPreservesCommentsAndTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := `# top comment
model = "gpt-5"

[other]
key = "value"

[mcp.servers.legacy]
command = "old"
`
	os.WriteFile(path, []byte(original), 0o600)
	servers := []Server{{
		ID:           "ctx7",
		Spec:         map[string]any{"type": "stdio", "command": "npx", "args": []any{"-y", "ctx7"}, "env": map[string]any{"TOKEN": "abc"}},
		EnabledCodex: true,
	}}
	if err := ApplyCodex(path, servers); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	txt := string(raw)
	if !strings.Contains(txt, "# top comment") {
		t.Error("注释应保留")
	}
	if !strings.Contains(txt, `model = "gpt-5"`) {
		t.Error("root key 应保留")
	}
	if !strings.Contains(txt, "[other]") || !strings.Contains(txt, `key = "value"`) {
		t.Error("无关表应保留")
	}
	if strings.Contains(txt, "[mcp.servers.legacy]") {
		t.Error("遗留 [mcp.servers] 应清理")
	}
	if !strings.Contains(txt, "[mcp_servers.ctx7]") || !strings.Contains(txt, `command = "npx"`) {
		t.Error("新段应写入")
	}
	if !strings.Contains(txt, "[mcp_servers.ctx7.env]") || !strings.Contains(txt, `TOKEN = "abc"`) {
		t.Error("env 子表应写入")
	}
	var m map[string]any
	if err := toml.Unmarshal(raw, &m); err != nil {
		t.Fatalf("产物应为合法 TOML: %v", err)
	}
}

func testService(t *testing.T) (*Service, *DAO) {
	t.Helper()
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	st, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("打开 store 失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	dao := NewDAO(st)
	return NewService(dao), dao
}

func TestServiceToggleAndDeleteSync(t *testing.T) {
	svc, _ := testService(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), ".claude.json")
	os.WriteFile(path, []byte("{}"), 0o600)
	if _, err := svc.CreateTarget(ctx, Target{Client: ClientClaude, ConfigPath: path, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := svc.UpsertServer(ctx, Server{ID: "ctx7", Spec: map[string]any{"type": "stdio", "command": "npx"}}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if gjson.GetBytes(raw, "mcpServers.ctx7").Exists() {
		t.Error("未启用 claude 不应写入")
	}
	if err := svc.ToggleClient(ctx, "ctx7", ClientClaude, true); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(path)
	if !gjson.GetBytes(raw, "mcpServers.ctx7").Exists() {
		t.Error("启用 claude 后应写入")
	}
	if err := svc.DeleteServer(ctx, "ctx7"); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(path)
	if gjson.GetBytes(raw, "mcpServers.ctx7").Exists() {
		t.Error("删除后应从文件移除")
	}
}

func TestServiceImportConflictNoOverwrite(t *testing.T) {
	svc, dao := testService(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), ".claude.json")
	os.WriteFile(path, []byte(`{"mcpServers":{"new1":{"command":"a"},"ctx7":{"command":"different"}}}`), 0o600)
	// 经 dao 直接登记 target（不触发自动对账，保住文件内容供导入读取）。
	tgt, err := dao.CreateTarget(ctx, Target{Client: ClientClaude, ConfigPath: path, Enabled: true}, "2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	// 已存 ctx7（spec=original，禁用）。
	if err := dao.UpsertServer(ctx, Server{ID: "ctx7", Spec: map[string]any{"type": "stdio", "command": "original"}}, "2026-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Import(ctx, tgt.ID); err != nil {
		t.Fatal(err)
	}
	n1, found, _ := dao.GetServer(ctx, "new1")
	if !found || !n1.EnabledClaude {
		t.Error("new1 应被导入并启用 claude")
	}
	c7, _, _ := dao.GetServer(ctx, "ctx7")
	if c7.Spec["command"] != "original" {
		t.Errorf("冲突不应覆盖已存 spec，实际 command=%v", c7.Spec["command"])
	}
	if !c7.EnabledClaude {
		t.Error("冲突应翻 claude 开关位为启用")
	}
}
