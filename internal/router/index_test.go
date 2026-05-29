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

func TestResolveFallsBackToOpenAIAPIPassthrough(t *testing.T) {
	idx := NewIndex(&config.Config{
		OpenAIAPI: []config.OpenAIAPIChannel{
			{
				Name:          "openai",
				BaseURL:       "https://api.openai.com",
				APIKeyEntries: []config.APIKeyEntry{{APIKey: "sk-openai"}},
			},
			{
				Name:          "disabled",
				Disabled:      true,
				BaseURL:       "https://disabled.example.com",
				APIKeyEntries: []config.APIKeyEntry{{APIKey: "sk-disabled"}},
			},
		},
	})

	hits := idx.Resolve("gpt-4.1")
	if len(hits) != 1 {
		t.Fatalf("Resolve() len = %d, want 1", len(hits))
	}
	if hits[0].ChannelName != "openai" {
		t.Fatalf("ChannelName = %q, want openai", hits[0].ChannelName)
	}
	if hits[0].UpstreamModelName != "gpt-4.1" {
		t.Fatalf("UpstreamModelName = %q, want gpt-4.1", hits[0].UpstreamModelName)
	}
	if models := idx.Models(); len(models) != 0 {
		t.Fatalf("Models() = %#v, want no enumerated models for pass-through-only channel", models)
	}
}

func TestResolveUsesExplicitOAuthBeforeOpenAIPassthrough(t *testing.T) {
	idx := NewIndex(&config.Config{
		OpenAIAPI: []config.OpenAIAPIChannel{
			{
				Name:          "openai",
				BaseURL:       "https://api.openai.com",
				APIKeyEntries: []config.APIKeyEntry{{APIKey: "sk-openai"}},
			},
		},
		ChatGPTOAuth: []config.ChatGPTOAuthChannel{
			{
				Name: "chatgpt",
				Models: []config.ModelEntry{
					{Name: "chatgpt-real", Alias: "gpt-4.1"},
				},
			},
		},
	})

	hits := idx.Resolve("gpt-4.1")
	if len(hits) != 1 {
		t.Fatalf("Resolve() len = %d, want 1", len(hits))
	}
	if hits[0].ChannelType != config.ChannelTypeChatGPTOAuth {
		t.Fatalf("ChannelType = %q, want chatgpt-oauth", hits[0].ChannelType)
	}
	if hits[0].UpstreamModelName != "chatgpt-real" {
		t.Fatalf("UpstreamModelName = %q, want explicit OAuth model", hits[0].UpstreamModelName)
	}
	if models := idx.Models(); len(models) != 1 || models[0] != "gpt-4.1" {
		t.Fatalf("Models() = %#v, want explicit OAuth alias", models)
	}
}

func TestResolveUsesOAuthExplicitModelsWithoutPassthrough(t *testing.T) {
	idx := NewIndex(&config.Config{
		ChatGPTOAuth: []config.ChatGPTOAuthChannel{
			{
				Name: "chatgpt",
				Models: []config.ModelEntry{
					{Name: "gpt-5-codex", Alias: "codex"},
				},
			},
		},
	})

	hits := idx.Resolve("codex")
	if len(hits) != 1 {
		t.Fatalf("Resolve(codex) len = %d, want 1", len(hits))
	}
	if hits[0].ChannelType != config.ChannelTypeChatGPTOAuth {
		t.Fatalf("ChannelType = %q, want %q", hits[0].ChannelType, config.ChannelTypeChatGPTOAuth)
	}
	if hits[0].UpstreamModelName != "gpt-5-codex" {
		t.Fatalf("UpstreamModelName = %q, want gpt-5-codex", hits[0].UpstreamModelName)
	}
	if hits := idx.Resolve("unlisted"); len(hits) != 0 {
		t.Fatalf("Resolve(unlisted) len = %d, want no OAuth pass-through", len(hits))
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
