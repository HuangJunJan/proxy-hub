package credstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPutGetRoundTrip 验证写入后可读回，且重新 Open 能从磁盘加载。
func TestPutGetRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("打开 credstore 失败: %v", err)
	}

	want := Cred{APIKey: "sk-test-123"}
	if err := s.Put(1, want); err != nil {
		t.Fatalf("Put 失败: %v", err)
	}
	got, ok := s.Get(1)
	if !ok || got != want {
		t.Errorf("Get(1) = %+v, %v；期望 %+v, true", got, ok, want)
	}
	_ = s.Close()

	// 重新打开：应从磁盘加载。
	s2, err := Open(dir)
	if err != nil {
		t.Fatalf("重新打开失败: %v", err)
	}
	defer func() { _ = s2.Close() }()
	got2, ok := s2.Get(1)
	if !ok || got2 != want {
		t.Errorf("重载后 Get(1) = %+v, %v；期望 %+v, true", got2, ok, want)
	}
}

// TestUpstreamCredShape 验证 upstream 凭证含 base_url，api_key 凭证省略 base_url。
func TestUpstreamCredShape(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	if err := s.Put(7, Cred{APIKey: "k", BaseURL: "https://up.example/v1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Put(8, Cred{APIKey: "k2"}); err != nil {
		t.Fatal(err)
	}

	raw7, _ := os.ReadFile(filepath.Join(dir, "7.json"))
	if !strings.Contains(string(raw7), "base_url") {
		t.Errorf("upstream 凭证应含 base_url，实际 %s", raw7)
	}
	raw8, _ := os.ReadFile(filepath.Join(dir, "8.json"))
	if strings.Contains(string(raw8), "base_url") {
		t.Errorf("api_key 凭证应省略 base_url，实际 %s", raw8)
	}
}

// TestDelete 验证删除后读不到且文件移除；删除不存在项不报错。
func TestDelete(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	if err := s.Put(3, Cred{APIKey: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(3); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	if _, ok := s.Get(3); ok {
		t.Error("删除后不应再读到")
	}
	if _, err := os.Stat(filepath.Join(dir, "3.json")); !os.IsNotExist(err) {
		t.Error("删除后文件应不存在")
	}
	if err := s.Delete(999); err != nil {
		t.Errorf("删除不存在项应成功，实际 %v", err)
	}
}

// TestLoadAllSkipsBad 验证启动加载时跳过坏 JSON 与非凭证文件，只加载合法项。
func TestLoadAllSkipsBad(t *testing.T) {
	dir := t.TempDir()
	// 合法。
	good, _ := json.Marshal(Cred{APIKey: "good"})
	if err := os.WriteFile(filepath.Join(dir, "10.json"), good, 0o600); err != nil {
		t.Fatal(err)
	}
	// 坏 JSON。
	if err := os.WriteFile(filepath.Join(dir, "11.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	// 非凭证文件名（应忽略）。
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := Open(dir)
	if err != nil {
		t.Fatalf("打开失败: %v", err)
	}
	defer func() { _ = s.Close() }()

	if c, ok := s.Get(10); !ok || c.APIKey != "good" {
		t.Errorf("合法凭证应被加载，实际 %+v, %v", c, ok)
	}
	if _, ok := s.Get(11); ok {
		t.Error("坏 JSON 不应被加载")
	}
}
