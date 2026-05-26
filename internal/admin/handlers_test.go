package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"proxy-hub/internal/auth"
	"proxy-hub/internal/config"
	"proxy-hub/internal/store"
)

func TestSetupWritesConfigAndIssuesSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	manager := config.NewManager(filepath.Join(t.TempDir(), "config.yaml"), nil)
	if err := manager.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	sessions := auth.NewSessionManagerWithSecret([]byte("01234567890123456789012345678901"))
	handler := NewHandler(manager, sessions)

	r := gin.New()
	handler.Register(r.Group("/api/admin"))

	body := bytes.NewBufferString(`{"username":"admin","password":"123456"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/setup", body)
	req.Header.Set("content-type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("setup status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode setup response: %v", err)
	}
	if payload.Token == "" {
		t.Fatal("setup token is empty")
	}
	if manager.SetupNeeded() {
		t.Fatal("SetupNeeded() = true, want false")
	}
	if len(rec.Result().Cookies()) == 0 {
		t.Fatal("setup did not set session cookie")
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	manager := config.NewManager(filepath.Join(t.TempDir(), "config.yaml"), nil)
	if err := manager.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	hash, err := auth.HashPassword("hunter22")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if err := manager.Save(func(cfg *config.Config) error {
		cfg.Admin = &config.AdminConfig{Username: "admin", PasswordHash: hash}
		cfg.APIKeys = []config.APIKeyConfig{{Token: "sk-proxy-hub-12345678901234567890"}}
		return nil
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	handler := NewHandler(manager, auth.NewSessionManagerWithSecret([]byte("01234567890123456789012345678901")))
	r := gin.New()
	handler.Register(r.Group("/api/admin"))

	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", bytes.NewBufferString(`{"username":"admin","password":"wrong"}`))
	req.Header.Set("content-type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("login status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminChannelsAndKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	manager := readyConfigManager(t)
	sessions := auth.NewSessionManagerWithSecret([]byte("01234567890123456789012345678901"))
	handler := NewHandler(manager, sessions)

	r := gin.New()
	handler.Register(r.Group("/api/admin"))
	cookie := sessionCookie(t, sessions)

	createBody := `{"name":"deepseek","base-url":"https://api.deepseek.com","api-key-entries":[{"api-key":"sk-test"}],"models":[{"name":"deepseek-chat","alias":"gpt-5.4"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/channels/openai-api", bytes.NewBufferString(createBody))
	req.Header.Set("content-type", "application/json")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create channel status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(manager.Snapshot().OpenAIAPI) != 1 {
		t.Fatalf("OpenAIAPI len = %d, want 1", len(manager.Snapshot().OpenAIAPI))
	}

	req = httptest.NewRequest(http.MethodPost, "/api/admin/keys", bytes.NewBufferString(`{"name":"cursor","notes":"local"}`))
	req.Header.Set("content-type", "application/json")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create key status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode create key response: %v", err)
	}
	if payload.Token == "" {
		t.Fatal("created token is empty")
	}

	req = httptest.NewRequest(http.MethodGet, "/api/admin/keys", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list keys status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(payload.Token)) {
		t.Fatalf("list keys should include token for admin copy action: %s", rec.Body.String())
	}
}

func TestAdminLogsAndStats(t *testing.T) {
	gin.SetMode(gin.TestMode)
	manager := readyConfigManager(t)
	sessions := auth.NewSessionManagerWithSecret([]byte("01234567890123456789012345678901"))
	db, err := store.OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "proxy-hub.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()
	if err := db.BatchInsert(context.Background(), []store.LogEntry{{
		TimestampMS:     1000,
		APIKeyTokenMask: "sk-...1234",
		ChannelName:     "openai",
		ChannelType:     "openai-api",
		DownstreamModel: "gpt-4o",
		StatusCode:      200,
		DurationMS:      10,
	}}); err != nil {
		t.Fatalf("BatchInsert() error = %v", err)
	}
	if err := db.UpsertHourly(context.Background(), []store.HourlyDelta{{
		ChannelName:     "openai",
		HourTimestampMS: timeNowHourForTest(),
		Requests:        1,
		Successes:       1,
		AvgDurationMS:   10,
	}}); err != nil {
		t.Fatalf("UpsertHourly() error = %v", err)
	}

	handler := NewHandler(manager, sessions, Dependencies{Logs: db, Stats: db})
	r := gin.New()
	handler.Register(r.Group("/api/admin"))
	cookie := sessionCookie(t, sessions)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/logs?channel=openai", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("logs status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"channelName":"openai"`)) {
		t.Fatalf("logs response missing channel: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/admin/stats/channels?window=24h", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("stats status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"requests":1`)) {
		t.Fatalf("stats response missing request count: %s", rec.Body.String())
	}
}

func TestAdminEmptyLogsAndStatsReturnArrays(t *testing.T) {
	gin.SetMode(gin.TestMode)
	manager := readyConfigManager(t)
	sessions := auth.NewSessionManagerWithSecret([]byte("01234567890123456789012345678901"))
	db, err := store.OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "proxy-hub.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	handler := NewHandler(manager, sessions, Dependencies{Logs: db, Stats: db})
	r := gin.New()
	handler.Register(r.Group("/api/admin"))
	cookie := sessionCookie(t, sessions)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/logs", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("logs status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"items":[]`)) {
		t.Fatalf("empty logs response did not return array: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/admin/stats/channels?window=24h", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("stats status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := bytes.TrimSpace(rec.Body.Bytes()); !bytes.Equal(got, []byte("[]")) {
		t.Fatalf("empty stats response = %s, want []", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/admin/stats/series?channel=openai&metric=requests&window=24h", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("series status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := bytes.TrimSpace(rec.Body.Bytes()); !bytes.Equal(got, []byte("[]")) {
		t.Fatalf("empty series response = %s, want []", got)
	}
}

func TestAdminEmptyChannelsReturnArrays(t *testing.T) {
	gin.SetMode(gin.TestMode)
	manager := readyConfigManager(t)
	sessions := auth.NewSessionManagerWithSecret([]byte("01234567890123456789012345678901"))
	handler := NewHandler(manager, sessions)

	r := gin.New()
	handler.Register(r.Group("/api/admin"))
	req := httptest.NewRequest(http.MethodGet, "/api/admin/channels", nil)
	req.AddCookie(sessionCookie(t, sessions))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("channels status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"openai-api":[]`)) {
		t.Fatalf("empty openai channels response = %s, want []", rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"chatgpt-oauth":[]`)) {
		t.Fatalf("empty oauth channels response = %s, want []", rec.Body.String())
	}
}

func readyConfigManager(t *testing.T) *config.Manager {
	t.Helper()
	manager := config.NewManager(filepath.Join(t.TempDir(), "config.yaml"), nil)
	if err := manager.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	hash, err := auth.HashPassword("hunter22")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if err := manager.Save(func(cfg *config.Config) error {
		cfg.Admin = &config.AdminConfig{Username: "admin", PasswordHash: hash}
		cfg.APIKeys = []config.APIKeyConfig{{Token: "sk-proxy-hub-12345678901234567890", Name: "default"}}
		return nil
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	return manager
}

func sessionCookie(t *testing.T, sessions *auth.SessionManager) *http.Cookie {
	t.Helper()
	token, err := sessions.Issue("admin", 30*24*time.Hour)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	return &http.Cookie{Name: auth.SessionCookieName, Value: token}
}

func timeNowHourForTest() int64 {
	const hourMS = int64(60 * 60 * 1000)
	return (time.Now().UnixMilli() / hourMS) * hourMS
}
