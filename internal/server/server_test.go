package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"proxy-hub/internal/auth"
	"proxy-hub/internal/config"
	"proxy-hub/internal/monitor"
	"proxy-hub/internal/store"
)

func TestV1ModelsRequiresBearer(t *testing.T) {
	router := NewRouter(Options{ConfigManager: testConfigManager(t, ""), Sessions: testSessions()})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload auth.OpenAIErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Error.Code != "invalid_api_key" {
		t.Fatalf("error code = %q", payload.Error.Code)
	}
}

func TestV1ModelsReturnsAliases(t *testing.T) {
	router := NewRouter(Options{ConfigManager: testConfigManager(t, ""), Sessions: testSessions()})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer sk-proxy-hub-test-token-1234567890")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"id":"gpt-5.4"`)) {
		t.Fatalf("models response missing alias: %s", rec.Body.String())
	}
}

func TestChatCompletionsRoutesAliasToUpstreamModel(t *testing.T) {
	var gotModel string
	var gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		var payload struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}
		gotModel = payload.Model
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-test","object":"chat.completion","choices":[]}`))
	}))
	defer upstream.Close()

	router := NewRouter(Options{ConfigManager: testConfigManager(t, upstream.URL), Sessions: testSessions()})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"gpt-5.4","messages":[]}`))
	req.Header.Set("Authorization", "Bearer sk-proxy-hub-test-token-1234567890")
	req.Header.Set("content-type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if gotAuth != "Bearer sk-upstream-test" {
		t.Fatalf("upstream Authorization = %q", gotAuth)
	}
	if gotModel != "deepseek-chat" {
		t.Fatalf("upstream model = %q, want deepseek-chat", gotModel)
	}
}

func TestRootChatCompletionsRouteSupportsBYOKBaseURLWithoutV1(t *testing.T) {
	var gotModel string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}
		gotModel = payload.Model
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-test","object":"chat.completion","choices":[]}`))
	}))
	defer upstream.Close()

	router := NewRouter(Options{ConfigManager: testConfigManager(t, upstream.URL), Sessions: testSessions()})
	req := httptest.NewRequest(http.MethodPost, "/chat/completions", bytes.NewBufferString(`{"model":"gpt-5.4","messages":[]}`))
	req.Header.Set("Authorization", "Bearer sk-proxy-hub-test-token-1234567890")
	req.Header.Set("content-type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if gotModel != "deepseek-chat" {
		t.Fatalf("upstream model = %q, want deepseek-chat", gotModel)
	}
}

func TestResponsesRoutesAliasToUpstreamModel(t *testing.T) {
	var gotModel string
	var gotInput string
	var gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("upstream path = %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		var payload struct {
			Model string `json:"model"`
			Input string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}
		gotModel = payload.Model
		gotInput = payload.Input
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp-test","object":"response","output":[]}`))
	}))
	defer upstream.Close()

	router := NewRouter(Options{ConfigManager: testConfigManager(t, upstream.URL), Sessions: testSessions()})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"model":"gpt-5.4","input":"hi"}`))
	req.Header.Set("Authorization", "Bearer sk-proxy-hub-test-token-1234567890")
	req.Header.Set("content-type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if gotAuth != "Bearer sk-upstream-test" {
		t.Fatalf("upstream Authorization = %q", gotAuth)
	}
	if gotModel != "deepseek-chat" {
		t.Fatalf("upstream model = %q, want deepseek-chat", gotModel)
	}
	if gotInput != "hi" {
		t.Fatalf("upstream input = %q, want hi", gotInput)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"object":"response"`)) {
		t.Fatalf("responses body was not relayed: %s", rec.Body.String())
	}
}

func TestRootResponsesRouteSupportsBYOKBaseURLWithoutV1(t *testing.T) {
	var gotModel string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("upstream path = %s", r.URL.Path)
		}
		var payload struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}
		gotModel = payload.Model
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp-test","object":"response","output":[]}`))
	}))
	defer upstream.Close()

	router := NewRouter(Options{ConfigManager: testConfigManager(t, upstream.URL), Sessions: testSessions()})
	req := httptest.NewRequest(http.MethodPost, "/responses", bytes.NewBufferString(`{"model":"gpt-5.4","input":"hi"}`))
	req.Header.Set("Authorization", "Bearer sk-proxy-hub-test-token-1234567890")
	req.Header.Set("content-type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if gotModel != "deepseek-chat" {
		t.Fatalf("upstream model = %q, want deepseek-chat", gotModel)
	}
}

func TestResponsesModelNotFoundReturnsOpenAIError(t *testing.T) {
	router := NewRouter(Options{ConfigManager: testConfigManager(t, ""), Sessions: testSessions()})

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"model":"missing-model","input":"hi"}`))
	req.Header.Set("Authorization", "Bearer sk-proxy-hub-test-token-1234567890")
	req.Header.Set("content-type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload auth.OpenAIErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Error.Code != "model_not_found" {
		t.Fatalf("error code = %q", payload.Error.Code)
	}
}

func TestChatCompletionsModelNotFoundLogHasNoUpstreamKeyIndex(t *testing.T) {
	db, monitorService, stopMonitor := testMonitor(t)
	defer stopMonitor()
	router := NewRouter(Options{ConfigManager: testConfigManager(t, ""), Sessions: testSessions(), Monitor: monitorService, Logs: db, Stats: db})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"missing-model","messages":[]}`))
	req.Header.Set("Authorization", "Bearer sk-proxy-hub-test-token-1234567890")
	req.Header.Set("content-type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload auth.OpenAIErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Error.Code != "model_not_found" {
		t.Fatalf("error code = %q", payload.Error.Code)
	}
	requireEventually(t, func() bool {
		logs, err := db.Query(context.Background(), store.QueryFilter{})
		return err == nil && len(logs) == 1
	})
	logs, err := db.Query(context.Background(), store.QueryFilter{})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if logs[0].UpstreamKeyIndex != nil {
		t.Fatalf("UpstreamKeyIndex = %v, want nil", *logs[0].UpstreamKeyIndex)
	}
}

func TestChatCompletionsFailsOverAndLogsUsage(t *testing.T) {
	firstCalls := 0
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstCalls++
		http.Error(w, "bad key", http.StatusUnauthorized)
	}))
	defer first.Close()

	secondCalls := 0
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondCalls++
		var payload struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode second upstream request: %v", err)
		}
		if payload.Model != "deepseek-chat" {
			t.Fatalf("second upstream model = %q", payload.Model)
		}
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ok","object":"chat.completion","choices":[],"usage":{"prompt_tokens":3,"completion_tokens":5,"total_tokens":8}}`))
	}))
	defer second.Close()

	manager := testConfigManagerWithChannels(t, []config.OpenAIAPIChannel{
		{
			Name:          "first",
			BaseURL:       first.URL,
			Priority:      10,
			APIKeyEntries: []config.APIKeyEntry{{APIKey: "sk-first"}},
			Models:        []config.ModelEntry{{Name: "deepseek-chat", Alias: "gpt-5.4"}},
		},
		{
			Name:          "second",
			BaseURL:       second.URL,
			Priority:      20,
			APIKeyEntries: []config.APIKeyEntry{{APIKey: "sk-second"}},
			Models:        []config.ModelEntry{{Name: "deepseek-chat", Alias: "gpt-5.4"}},
		},
	}, func(cfg *config.Config) {
		cfg.Scheduler.CircuitFailureThreshold = 1
		cfg.RequestLog.BodyMode = config.BodyModeAlways
	})
	db, monitorService, stopMonitor := testMonitor(t)
	defer stopMonitor()
	router := NewRouter(Options{ConfigManager: manager, Sessions: testSessions(), Monitor: monitorService, Logs: db, Stats: db})

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hi"}],"reasoning_effort":"high"}`))
		req.Header.Set("Authorization", "Bearer sk-proxy-hub-test-token-1234567890")
		req.Header.Set("content-type", "application/json")
		req.Header.Set("User-Agent", "proxy-hub-test-agent")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d status = %d body=%s", i+1, rec.Code, rec.Body.String())
		}
	}

	if firstCalls != 1 {
		t.Fatalf("first upstream calls = %d, want 1", firstCalls)
	}
	if secondCalls != 2 {
		t.Fatalf("second upstream calls = %d, want 2", secondCalls)
	}
	requireEventually(t, func() bool {
		logs, err := db.Query(context.Background(), store.QueryFilter{ChannelName: "second"})
		return err == nil && len(logs) == 2
	})
	logs, err := db.Query(context.Background(), store.QueryFilter{ChannelName: "second"})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	firstLog := logs[len(logs)-1]
	if firstLog.Attempts != 2 {
		t.Fatalf("first logged attempts = %d, want 2", firstLog.Attempts)
	}
	if firstLog.TotalTokens == nil || *firstLog.TotalTokens != 8 {
		t.Fatalf("first logged total tokens = %v, want 8", firstLog.TotalTokens)
	}
	if firstLog.Endpoint != "/v1/chat/completions" {
		t.Fatalf("first logged endpoint = %q, want /v1/chat/completions", firstLog.Endpoint)
	}
	if firstLog.RequestType != "chat.completions" {
		t.Fatalf("first logged request type = %q, want chat.completions", firstLog.RequestType)
	}
	if firstLog.ReasoningEffort != "high" {
		t.Fatalf("first logged reasoning effort = %q, want high", firstLog.ReasoningEffort)
	}
	if firstLog.BillingMode != "token" {
		t.Fatalf("first logged billing mode = %q, want token", firstLog.BillingMode)
	}
	if firstLog.UserAgent != "proxy-hub-test-agent" {
		t.Fatalf("first logged user agent = %q, want proxy-hub-test-agent", firstLog.UserAgent)
	}
	if firstLog.FirstTokenMS == nil {
		t.Fatalf("first logged first token ms = nil, want measured value")
	}
	if !bytes.Contains(firstLog.RequestBody, []byte(`"gpt-5.4"`)) || !bytes.Contains(firstLog.ResponseBody, []byte(`"usage"`)) {
		t.Fatalf("bodyMode=always did not persist bodies: request=%s response=%s", string(firstLog.RequestBody), string(firstLog.ResponseBody))
	}
	summaries, err := db.QueryChannelSummary(context.Background(), store.TimeWindow{})
	if err != nil {
		t.Fatalf("QueryChannelSummary() error = %v", err)
	}
	if len(summaries) != 1 || summaries[0].TotalTokens != 16 {
		t.Fatalf("summaries = %+v, want total tokens 16", summaries)
	}
}

func TestSPAFallbackServesIndexButNotAPIRoutes(t *testing.T) {
	router := NewRouter(Options{ConfigManager: testConfigManager(t, ""), Sessions: testSessions()})

	req := httptest.NewRequest(http.MethodGet, "/channels/deepseek", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("SPA status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`<div id="root"></div>`)) {
		t.Fatalf("SPA fallback did not serve index.html: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/unknown", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("API unknown status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func testConfigManager(t *testing.T, upstreamURL string) *config.Manager {
	t.Helper()
	if upstreamURL == "" {
		upstreamURL = "https://api.example.com"
	}
	manager := config.NewManager(filepath.Join(t.TempDir(), "config.yaml"), nil)
	if err := manager.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := manager.Save(func(cfg *config.Config) error {
		cfg.Admin = &config.AdminConfig{Username: "admin", PasswordHash: "$argon2id$v=19$m=65536,t=3,p=2$abc$def"}
		cfg.APIKeys = []config.APIKeyConfig{
			{Token: "sk-proxy-hub-test-token-1234567890", Name: "test"},
		}
		cfg.OpenAIAPI = []config.OpenAIAPIChannel{
			{
				Name:    "deepseek",
				BaseURL: upstreamURL,
				APIKeyEntries: []config.APIKeyEntry{
					{APIKey: "sk-upstream-test"},
				},
				Models: []config.ModelEntry{
					{Name: "deepseek-chat", Alias: "gpt-5.4"},
				},
			},
		}
		return nil
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	return manager
}

func testConfigManagerWithChannels(t *testing.T, channels []config.OpenAIAPIChannel, mutate func(*config.Config)) *config.Manager {
	t.Helper()
	manager := config.NewManager(filepath.Join(t.TempDir(), "config.yaml"), nil)
	if err := manager.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := manager.Save(func(cfg *config.Config) error {
		cfg.Admin = &config.AdminConfig{Username: "admin", PasswordHash: "$argon2id$v=19$m=65536,t=3,p=2$abc$def"}
		cfg.APIKeys = []config.APIKeyConfig{
			{Token: "sk-proxy-hub-test-token-1234567890", Name: "test"},
		}
		cfg.OpenAIAPI = channels
		if mutate != nil {
			mutate(cfg)
		}
		return nil
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	return manager
}

func testMonitor(t *testing.T) (*store.SQLiteStore, *monitor.Service, func()) {
	t.Helper()
	db, err := store.OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "proxy-hub.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	service := monitor.NewService(db, db, nil, monitor.Options{BatchSize: 1, FlushInterval: time.Hour})
	go service.Run(ctx, monitor.Options{BatchSize: 1, FlushInterval: time.Hour})
	return db, service, func() {
		cancel()
		db.Close()
	}
}

func requireEventually(t *testing.T, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}

func testSessions() *auth.SessionManager {
	return auth.NewSessionManagerWithSecret([]byte("01234567890123456789012345678901"))
}
