package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/huangjunjan/proxy-hub/internal/fileio"
)

// ApplyClaude 把启用集投影进 ~/.claude.json 的 mcpServers：逐 id set（启用）或 delete（禁用），
// 经 sjson 只触碰 mcpServers 子树，**其余顶层键（含 projects）字节级保留**。未被 proxy-hub 管理的
// mcpServers 条目也保留（只增删本注册表内的 id）。wrapWindows 时对 stdio 的 npx/node 等做 cmd /c 包装。
func ApplyClaude(path string, all []Server, wrapWindows bool) error {
	return fileio.UpdateFile(path, func(current []byte) ([]byte, error) {
		out := current
		if len(bytes.TrimSpace(out)) == 0 {
			out = []byte("{}")
		}
		var err error
		for _, s := range all {
			p := "mcpServers." + escapeJSONKey(s.ID)
			if s.EnabledClaude {
				out, err = sjson.SetBytes(out, p, claudeObject(s, wrapWindows))
			} else {
				out, err = sjson.DeleteBytes(out, p)
			}
			if err != nil {
				return nil, fmt.Errorf("写入 mcpServers.%s 失败: %w", s.ID, err)
			}
		}
		return out, nil
	})
}

// ReadClaude 解析 ~/.claude.json 的 mcpServers 为 Server 列表（导入用；标记 EnabledClaude）。
func ReadClaude(path string) ([]Server, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	servers := []Server{}
	gjson.GetBytes(raw, "mcpServers").ForEach(func(key, val gjson.Result) bool {
		spec := map[string]any{}
		_ = json.Unmarshal([]byte(val.Raw), &spec)
		servers = append(servers, Server{ID: key.String(), Spec: spec, EnabledClaude: true})
		return true
	})
	return servers, nil
}

// claudeObject 构造写入 mcpServers.<id> 的对象（透传 spec 全部字段；可选 Windows cmd /c 包装）。
func claudeObject(s Server, wrapWindows bool) map[string]any {
	obj := make(map[string]any, len(s.Spec))
	for k, v := range s.Spec {
		obj[k] = v
	}
	if wrapWindows && SpecType(obj) == "stdio" {
		wrapCmdC(obj)
	}
	return obj
}

// wrapCmdC 把 stdio 的 npx/npm/node 等命令包装为 `cmd /c <command> <args...>`（Windows 需要）。
func wrapCmdC(obj map[string]any) {
	cmd, _ := obj["command"].(string)
	if !isWrappable(cmd) {
		return
	}
	args := toAnySlice(obj["args"])
	newArgs := make([]any, 0, len(args)+2)
	newArgs = append(newArgs, "/c", cmd)
	newArgs = append(newArgs, args...)
	obj["command"] = "cmd"
	obj["args"] = newArgs
}

// isWrappable 报告命令是否需 cmd /c 包装（Windows 上这些是 .cmd shim，须经 cmd 调用）。
func isWrappable(cmd string) bool {
	if cmd == "" {
		return false
	}
	base := strings.ToLower(filepath.Base(cmd))
	base = strings.TrimSuffix(base, ".exe")
	base = strings.TrimSuffix(base, ".cmd")
	switch base {
	case "npx", "npm", "node", "pnpm", "yarn", "bunx", "bun", "deno":
		return true
	}
	return false
}

func toAnySlice(v any) []any {
	switch s := v.(type) {
	case []any:
		return s
	case []string:
		out := make([]any, len(s))
		for i, e := range s {
			out[i] = e
		}
		return out
	}
	return nil
}

// escapeJSONKey 转义 sjson 路径中的特殊字符（点/通配），使含点的 id 也能精确寻址。
func escapeJSONKey(k string) string {
	r := strings.NewReplacer(".", `\.`, "*", `\*`, "?", `\?`)
	return r.Replace(k)
}
