package fileio

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestUpdateFileRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f.json")
	if err := UpdateFile(p, func(cur []byte) ([]byte, error) {
		if len(cur) != 0 {
			t.Fatalf("新建时 current 应为空，实际 %q", cur)
		}
		return []byte("v1"), nil
	}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "v1" {
		t.Fatalf("内容 = %q，期望 v1", b)
	}
	// 覆写：current 应为 v1，并生成 .bak。
	if err := UpdateFile(p, func(cur []byte) ([]byte, error) {
		if string(cur) != "v1" {
			t.Fatalf("覆写时 current 应为 v1，实际 %q", cur)
		}
		return []byte("v2"), nil
	}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "v2" {
		t.Fatalf("覆写后 = %q，期望 v2", b)
	}
	if b, err := os.ReadFile(p + ".bak"); err != nil || string(b) != "v1" {
		t.Fatalf(".bak 应含 v1，实际 %q err=%v", b, err)
	}
}

func TestUpdateFileBakOnlyOnce(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f.json")
	set := func(v string) {
		_ = UpdateFile(p, func([]byte) ([]byte, error) { return []byte(v), nil })
	}
	set("v1")
	set("v2")
	set("v3")
	if b, _ := os.ReadFile(p + ".bak"); string(b) != "v1" {
		t.Fatalf(".bak 应仅首次生成（v1），实际 %q", b)
	}
}

func TestUpdateFileSkipsMissingParent(t *testing.T) {
	p := filepath.Join(t.TempDir(), "missing-dir", "f.json")
	if err := UpdateFile(p, func([]byte) ([]byte, error) { return []byte("x"), nil }); !errors.Is(err, ErrParentMissing) {
		t.Fatalf("应返回 ErrParentMissing，实际 %v", err)
	}
}

func TestUpdateFileSerializes(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f")
	_ = UpdateFile(p, func([]byte) ([]byte, error) { return []byte("0"), nil })
	var concurrent, violated int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = UpdateFile(p, func(cur []byte) ([]byte, error) {
				if atomic.AddInt32(&concurrent, 1) != 1 {
					atomic.StoreInt32(&violated, 1)
				}
				time.Sleep(time.Millisecond)
				atomic.AddInt32(&concurrent, -1)
				return cur, nil
			})
		}()
	}
	wg.Wait()
	if violated != 0 {
		t.Fatal("UpdateFile 未能串行化同 path 的并发执行")
	}
}
