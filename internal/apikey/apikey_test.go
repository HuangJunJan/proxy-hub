package apikey

import (
	"errors"
	"testing"
)

func TestHashKeyDeterministic(t *testing.T) {
	h1 := HashKey("sk-ph-abc")
	h2 := HashKey("sk-ph-abc")
	if h1 != h2 {
		t.Fatalf("同一明文哈希应一致")
	}
	if len(h1) != 64 {
		t.Fatalf("sha256 hex 长度应为 64，实际 %d", len(h1))
	}
	if HashKey("other") == h1 {
		t.Fatalf("不同明文哈希应不同")
	}
}

func TestGenerateKeyUnique(t *testing.T) {
	a, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	b, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatalf("两次生成应不同")
	}
	if len(a) <= len(keyPrefix) || a[:len(keyPrefix)] != keyPrefix {
		t.Fatalf("应带前缀 %q: %q", keyPrefix, a)
	}
}

func TestCacheLookupPositiveNegativeAndCount(t *testing.T) {
	calls := 0
	load := func(hash string) (KeyMeta, bool, error) {
		calls++
		if hash == "known" {
			return KeyMeta{ID: 7, Group: "team", Enabled: true}, true, nil
		}
		return KeyMeta{}, false, nil
	}
	c := NewCache(load)

	// 正缓存：首次回源，二次命中缓存（calls 不再增加）。
	m, ok := c.Lookup("known")
	if !ok || m.ID != 7 || m.Group != "team" {
		t.Fatalf("应命中 known: %+v ok=%v", m, ok)
	}
	if _, ok := c.Lookup("known"); !ok {
		t.Fatalf("二次应仍命中")
	}
	if calls != 1 {
		t.Fatalf("正缓存应只回源一次，实际 %d", calls)
	}

	// 负缓存：首次回源，二次命中负缓存。
	if _, ok := c.Lookup("missing"); ok {
		t.Fatalf("missing 不应命中")
	}
	if _, ok := c.Lookup("missing"); ok {
		t.Fatalf("missing 二次仍不应命中")
	}
	if calls != 2 {
		t.Fatalf("负缓存应只回源一次，实际 %d", calls)
	}

	// 失效后重新回源。
	c.Invalidate()
	if _, ok := c.Lookup("known"); !ok {
		t.Fatalf("失效后应能重新回源命中")
	}
	if calls != 3 {
		t.Fatalf("失效后应再回源一次，实际 %d", calls)
	}
}

func TestCacheLoadErrorFailClosed(t *testing.T) {
	load := func(hash string) (KeyMeta, bool, error) {
		return KeyMeta{}, false, errors.New("db down")
	}
	c := NewCache(load)
	if _, ok := c.Lookup("x"); ok {
		t.Fatalf("回源出错应 fail-closed 返回未命中")
	}
	// 出错不进负缓存：下次应重试（再次回源）。
	calls := 0
	c2 := NewCache(func(hash string) (KeyMeta, bool, error) {
		calls++
		return KeyMeta{}, false, errors.New("db down")
	})
	_, _ = c2.Lookup("x")
	_, _ = c2.Lookup("x")
	if calls != 2 {
		t.Fatalf("错误不应缓存，应每次重试，实际回源 %d 次", calls)
	}
}
