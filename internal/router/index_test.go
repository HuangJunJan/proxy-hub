package router

import (
	"testing"
	"time"

	"proxy-hub/internal/config"
)

func TestResolveUsesModelAlias(t *testing.T) {
	idx := NewIndex(&config.Config{
		OpenAIAPI: []config.OpenAIAPIChannel{
			{
				Name:    "deepseek",
				BaseURL: "https://api.deepseek.com",
				APIKeyEntries: []config.APIKeyEntry{
					{APIKey: "sk-test"},
				},
				Models: []config.ModelEntry{
					{Name: "deepseek-chat", Alias: "gpt-5.4"},
				},
			},
		},
	})

	hits := idx.Resolve("GPT-5.4")
	if len(hits) != 1 {
		t.Fatalf("Resolve() len = %d, want 1", len(hits))
	}
	if hits[0].UpstreamModelName != "deepseek-chat" {
		t.Fatalf("UpstreamModelName = %q", hits[0].UpstreamModelName)
	}
	if hits[0].Timeout != time.Duration(config.DefaultTimeoutSec)*time.Second {
		t.Fatalf("Timeout = %s", hits[0].Timeout)
	}
}

func TestModelsReturnsEnabledAliasUnion(t *testing.T) {
	idx := NewIndex(&config.Config{
		OpenAIAPI: []config.OpenAIAPIChannel{
			{
				Name:          "a",
				BaseURL:       "https://a.example.com",
				APIKeyEntries: []config.APIKeyEntry{{APIKey: "sk-a"}},
				Models: []config.ModelEntry{
					{Name: "gpt-4o"},
					{Name: "real", Alias: "alias"},
				},
			},
			{
				Name:          "disabled",
				Disabled:      true,
				BaseURL:       "https://b.example.com",
				APIKeyEntries: []config.APIKeyEntry{{APIKey: "sk-b"}},
				Models:        []config.ModelEntry{{Name: "hidden"}},
			},
		},
	})
	got := idx.Models()
	want := []string{"alias", "gpt-4o"}
	if len(got) != len(want) {
		t.Fatalf("Models() = %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Models()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
