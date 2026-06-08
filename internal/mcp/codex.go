package mcp

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/huangjunjan/proxy-hub/internal/fileio"
)

// ApplyCodex 把启用集投影进 ~/.codex/config.toml：用文本段手术删除所有现有 [mcp_servers*] 与遗留
// [mcp.servers*] 段，再把启用的 server 序列化为新 [mcp_servers.<id>] 段追加。**[mcp_servers] 之外的
// 表与注释逐字保留**（go-toml 全量往返会丢注释，故不用之写）。
func ApplyCodex(path string, all []Server) error {
	return fileio.UpdateFile(path, func(current []byte) ([]byte, error) {
		body := stripMcpSections(string(current))
		body = strings.TrimRight(body, "\n")
		var b strings.Builder
		b.WriteString(body)
		for _, s := range all {
			if !s.EnabledCodex {
				continue
			}
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString(codexServerTOML(s))
		}
		out := b.String()
		if out != "" && !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		return []byte(out), nil
	})
}

// ReadCodex 解析 config.toml 的 [mcp_servers] 为 Server 列表（导入用；标记 EnabledCodex）。
func ReadCodex(path string) ([]Server, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var root map[string]any
	if err := toml.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("解析 config.toml 失败: %w", err)
	}
	table, _ := root["mcp_servers"].(map[string]any)
	servers := []Server{}
	for id, v := range table {
		spec, _ := v.(map[string]any)
		if spec == nil {
			continue
		}
		if _, ok := spec["type"]; !ok {
			if _, hasURL := spec["url"]; hasURL {
				spec["type"] = "http"
			} else {
				spec["type"] = "stdio"
			}
		}
		servers = append(servers, Server{ID: id, Spec: spec, EnabledCodex: true})
	}
	sort.Slice(servers, func(i, j int) bool { return servers[i].ID < servers[j].ID })
	return servers, nil
}

// stripMcpSections 按表头逐行切割，删除所有 [mcp_servers*] / [mcp.servers*] 段（含其内容行），其余原样保留。
func stripMcpSections(text string) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	dropping := false
	for _, line := range lines {
		if name, ok := tableHeaderName(line); ok {
			dropping = isMcpSection(name)
		}
		if !dropping {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// tableHeaderName 若该行是 TOML 表头（[x] 或 [[x]]）则返回括号内点分名。
func tableHeaderName(line string) (string, bool) {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, "[") {
		return "", false
	}
	t = strings.TrimPrefix(t, "[")
	t = strings.TrimPrefix(t, "[")
	idx := strings.Index(t, "]")
	if idx < 0 {
		return "", false
	}
	return strings.TrimSpace(t[:idx]), true
}

func isMcpSection(name string) bool {
	return name == "mcp_servers" || strings.HasPrefix(name, "mcp_servers.") ||
		name == "mcp.servers" || strings.HasPrefix(name, "mcp.servers.")
}

// codexServerTOML 把一个 server 序列化为 [mcp_servers.<id>] 段（含 env/http_headers 子表 + 扩展字段透传）。
func codexServerTOML(s Server) string {
	id := tomlKey(s.ID)
	var b strings.Builder
	fmt.Fprintf(&b, "[mcp_servers.%s]\n", id)
	spec := s.Spec
	emitted := map[string]bool{"type": true, "env": true, "headers": true, "http_headers": true}
	if SpecType(spec) == "stdio" {
		if cmd, ok := spec["command"].(string); ok {
			fmt.Fprintf(&b, "command = %s\n", tomlString(cmd))
			emitted["command"] = true
		}
		if args := toAnySlice(spec["args"]); args != nil {
			fmt.Fprintf(&b, "args = %s\n", tomlArray(args))
			emitted["args"] = true
		}
		if cwd, ok := spec["cwd"].(string); ok && cwd != "" {
			fmt.Fprintf(&b, "cwd = %s\n", tomlString(cwd))
			emitted["cwd"] = true
		}
	} else {
		if url, ok := spec["url"].(string); ok {
			fmt.Fprintf(&b, "url = %s\n", tomlString(url))
			emitted["url"] = true
		}
	}
	// 扩展字段透传（标量/数组），跳过已写与表字段。
	for _, k := range sortedKeys(spec) {
		if emitted[k] {
			continue
		}
		if rendered, ok := tomlScalarOrArray(spec[k]); ok {
			fmt.Fprintf(&b, "%s = %s\n", tomlKey(k), rendered)
		}
	}
	if env := toStringMap(spec["env"]); len(env) > 0 {
		fmt.Fprintf(&b, "\n[mcp_servers.%s.env]\n", id)
		for _, k := range sortedKeys2(env) {
			fmt.Fprintf(&b, "%s = %s\n", tomlKey(k), tomlString(env[k]))
		}
	}
	headers := toStringMap(spec["http_headers"])
	if len(headers) == 0 {
		headers = toStringMap(spec["headers"])
	}
	if len(headers) > 0 {
		fmt.Fprintf(&b, "\n[mcp_servers.%s.http_headers]\n", id)
		for _, k := range sortedKeys2(headers) {
			fmt.Fprintf(&b, "%s = %s\n", tomlKey(k), tomlString(headers[k]))
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// ---- TOML 序列化小工具 ----

// tomlString 输出 TOML 基本字符串（strconv.Quote 与 TOML 基本串转义高度一致）。
func tomlString(s string) string { return strconv.Quote(s) }

// tomlKey 裸键（[A-Za-z0-9_-]+）原样，否则加引号。
func tomlKey(k string) string {
	if k == "" {
		return `""`
	}
	for _, r := range k {
		if !(r == '_' || r == '-' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return strconv.Quote(k)
		}
	}
	return k
}

func tomlArray(items []any) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		if r, ok := tomlScalar(it); ok {
			parts = append(parts, r)
		}
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func tomlScalar(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return tomlString(x), true
	case bool:
		return strconv.FormatBool(x), true
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10), true
		}
		return strconv.FormatFloat(x, 'g', -1, 64), true
	case int64:
		return strconv.FormatInt(x, 10), true
	case int:
		return strconv.Itoa(x), true
	}
	return "", false
}

func tomlScalarOrArray(v any) (string, bool) {
	if arr := toAnySlice(v); arr != nil {
		return tomlArray(arr), true
	}
	return tomlScalar(v)
}

// toStringMap 把 spec 的 env/headers（map[string]any）转为 map[string]string（值字符串化）。
func toStringMap(v any) map[string]string {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, val := range m {
		if s, ok := val.(string); ok {
			out[k] = s
		} else {
			out[k] = fmt.Sprintf("%v", val)
		}
	}
	return out
}

func sortedKeys(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func sortedKeys2(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
