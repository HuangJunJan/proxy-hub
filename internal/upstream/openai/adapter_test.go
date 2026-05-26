package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"proxy-hub/internal/upstream"
)

func TestChatReplacesModelAndSetsAuth(t *testing.T) {
	var gotModel string
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		var payload struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		gotModel = payload.Model
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-test","choices":[]}`))
	}))
	defer server.Close()

	adapter := New(server.Client())
	resp, err := adapter.Chat(context.Background(), upstream.ChatRequest{
		BaseURL:           server.URL,
		APIKey:            "sk-test",
		UpstreamModelName: "deepseek-chat",
		OriginalBody:      []byte(`{"model":"gpt-5.4","messages":[]}`),
		Timeout:           time.Second,
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	defer resp.Body.Close()
	if gotAuth != "Bearer sk-test" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotModel != "deepseek-chat" {
		t.Fatalf("model = %q, want deepseek-chat", gotModel)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestChatStreamAcceptHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Fatalf("Accept = %q", r.Header.Get("Accept"))
		}
		w.Header().Set("content-type", "text/event-stream")
		_, _ = w.Write([]byte("data: {}\n\n"))
	}))
	defer server.Close()

	adapter := New(server.Client())
	resp, err := adapter.Chat(context.Background(), upstream.ChatRequest{
		BaseURL:           server.URL,
		APIKey:            "sk-test",
		UpstreamModelName: "gpt-4o",
		OriginalBody:      []byte(`{"model":"gpt-4o","stream":true}`),
		Stream:            true,
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(data) != "data: {}\n\n" {
		t.Fatalf("body = %q", string(data))
	}
}

func TestModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"gpt-4o"},{"id":"gpt-4o-mini"}]}`))
	}))
	defer server.Close()

	models, err := New(server.Client()).Models(context.Background(), upstream.ModelsRequest{
		BaseURL: server.URL,
		APIKey:  "sk-test",
	})
	if err != nil {
		t.Fatalf("Models() error = %v", err)
	}
	if len(models) != 2 || models[0] != "gpt-4o" || models[1] != "gpt-4o-mini" {
		t.Fatalf("Models() = %#v", models)
	}
}

func TestReplaceModelRejectsInvalidJSON(t *testing.T) {
	if _, err := ReplaceModel([]byte(`{`), "gpt-4o"); err == nil {
		t.Fatal("ReplaceModel() error = nil, want error")
	}
}
