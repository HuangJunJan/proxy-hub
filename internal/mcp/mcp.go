// Package mcp 实现 MCP 服务器的单一事实源（SSOT）与向 Codex / Claude Code 客户端配置的投影。
//
// SSOT 存于 mcp_servers 表；每服务器带 per-client 启用位图。同步把「启用集」投影进显式登记的
// sync target 文件（~/.codex/config.toml、~/.claude.json），经 internal/fileio 原子写并保留无关内容
// （NFR-5）。Claude 用 sjson 只改 mcpServers 键；Codex 用文本段手术只改 [mcp_servers] 段。
package mcp

import (
	"errors"
	"fmt"
)

// Client 是支持的客户端类型。
const (
	ClientCodex  = "codex"
	ClientClaude = "claude"
)

// Server 是一个 MCP 服务器的领域表示（SSOT 行）。
type Server struct {
	ID            string
	Name          string
	Spec          map[string]any // 规范宽松 spec：stdio {type,command,args,env,cwd} / http|sse {type,url,headers}；保留未知字段
	Description   string
	Homepage      string
	Docs          string
	Tags          []string
	EnabledCodex  bool
	EnabledClaude bool
}

// Target 是一个显式登记的可写客户端配置文件。
type Target struct {
	ID             int64
	Client         string // codex | claude
	ConfigPath     string // 绝对路径
	Label          string
	Enabled        bool
	LastSyncedAt   string
	LastSyncStatus string
}

// SpecType 返回 spec 的 type（缺省 stdio）。
func SpecType(spec map[string]any) string {
	if t, ok := spec["type"].(string); ok && t != "" {
		return t
	}
	return "stdio"
}

// Validate 对 spec 做宽松校验：须为对象；type ∈ {stdio,http,sse}；stdio 须有 command，http/sse 须有 url。
// 不裁剪任何字段（保留未知字段，往返不丢信息）。
func Validate(spec map[string]any) error {
	if spec == nil {
		return errors.New("spec 不能为空")
	}
	switch SpecType(spec) {
	case "stdio":
		if s, _ := spec["command"].(string); s == "" {
			return errors.New("stdio 类型的 spec 须有非空 command")
		}
	case "http", "sse":
		if s, _ := spec["url"].(string); s == "" {
			return errors.New("http/sse 类型的 spec 须有非空 url")
		}
	default:
		return fmt.Errorf("不支持的 MCP type: %q（应为 stdio|http|sse）", SpecType(spec))
	}
	return nil
}
