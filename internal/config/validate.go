package config

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var tokenPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type ValidationError struct {
	Problems []string
}

func (e *ValidationError) Error() string {
	return "invalid config: " + strings.Join(e.Problems, "; ")
}

func Validate(cfg *Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	var problems []string
	add := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}

	if cfg.Server.Port < 0 || cfg.Server.Port > 65535 {
		add("server.port must be between 1 and 65535")
	}
	if cfg.Admin != nil {
		if strings.TrimSpace(cfg.Admin.Username) == "" {
			add("admin.username is required when admin is configured")
		}
		if strings.TrimSpace(cfg.Admin.PasswordHash) == "" {
			add("admin.password-hash is required when admin is configured")
		}
	}

	seenTokens := map[string]struct{}{}
	for i, key := range cfg.APIKeys {
		token := strings.TrimSpace(key.Token)
		if token == "" {
			add("api-keys[%d].token is required", i)
			continue
		}
		if len(token) < 32 {
			add("api-keys[%d].token must be at least 32 characters", i)
		}
		if !tokenPattern.MatchString(token) {
			add("api-keys[%d].token contains invalid characters", i)
		}
		if _, ok := seenTokens[token]; ok {
			add("api-keys[%d].token duplicates another token", i)
		}
		seenTokens[token] = struct{}{}
	}

	validateOpenAIChannels(cfg.OpenAIAPI, add)
	validateOAuthChannels(cfg.ChatGPTOAuth, add)
	validateRequestLog(cfg.RequestLog, add)
	validateScheduler(cfg.Scheduler, add)
	validateCORS(cfg.CORS, add)

	if len(problems) > 0 {
		return &ValidationError{Problems: problems}
	}
	return nil
}

func validateOpenAIChannels(channels []OpenAIAPIChannel, add func(string, ...any)) {
	seenNames := map[string]struct{}{}
	for i, ch := range channels {
		name := strings.TrimSpace(ch.Name)
		if name == "" {
			add("openai-api[%d].name is required", i)
		} else if _, ok := seenNames[name]; ok {
			add("openai-api[%d].name duplicates %q", i, name)
		}
		seenNames[name] = struct{}{}

		if strings.TrimSpace(ch.BaseURL) == "" {
			add("openai-api[%d].base-url is required", i)
		} else if err := validateHTTPURL(ch.BaseURL); err != nil {
			add("openai-api[%d].base-url %v", i, err)
		}
		if ch.Priority < 0 {
			add("openai-api[%d].priority cannot be negative", i)
		}
		if ch.TimeoutSec < 0 {
			add("openai-api[%d].timeout-sec cannot be negative", i)
		}
		if len(ch.APIKeyEntries) == 0 {
			add("openai-api[%d].api-key-entries must contain at least one key", i)
		}
		for j, entry := range ch.APIKeyEntries {
			if strings.TrimSpace(entry.APIKey) == "" {
				add("openai-api[%d].api-key-entries[%d].api-key is required", i, j)
			}
			if strings.TrimSpace(entry.ProxyURL) != "" {
				if err := validateHTTPURL(entry.ProxyURL); err != nil {
					add("openai-api[%d].api-key-entries[%d].proxy-url %v", i, j, err)
				}
			}
		}
		validateModels(fmt.Sprintf("openai-api[%d]", i), ch.Models, add)
	}
}

func validateOAuthChannels(channels []ChatGPTOAuthChannel, add func(string, ...any)) {
	seenNames := map[string]struct{}{}
	for i, ch := range channels {
		name := strings.TrimSpace(ch.Name)
		if name == "" {
			add("chatgpt-oauth[%d].name is required", i)
		} else if _, ok := seenNames[name]; ok {
			add("chatgpt-oauth[%d].name duplicates %q", i, name)
		}
		seenNames[name] = struct{}{}
		if ch.TimeoutSec < 0 {
			add("chatgpt-oauth[%d].timeout-sec cannot be negative", i)
		}
		if strings.TrimSpace(ch.OAuth.AccessToken) == "" {
			add("chatgpt-oauth[%d].oauth.access-token is required", i)
		}
		if strings.TrimSpace(ch.OAuth.RefreshToken) == "" {
			add("chatgpt-oauth[%d].oauth.refresh-token is required", i)
		}
		if ch.OAuth.ExpiresAt.IsZero() {
			add("chatgpt-oauth[%d].oauth.expires-at is required", i)
		}
		validateModels(fmt.Sprintf("chatgpt-oauth[%d]", i), ch.Models, add)
	}
}

func validateModels(prefix string, models []ModelEntry, add func(string, ...any)) {
	if len(models) == 0 {
		add("%s.models must contain at least one model", prefix)
		return
	}
	seen := map[string]struct{}{}
	for i, model := range models {
		name := strings.TrimSpace(model.Name)
		alias := strings.TrimSpace(model.Alias)
		if name == "" {
			add("%s.models[%d].name is required", prefix, i)
		}
		key := name + "\x00" + alias
		if _, ok := seen[key]; ok {
			add("%s.models[%d] duplicates model name/alias pair", prefix, i)
		}
		seen[key] = struct{}{}
	}
}

func validateRequestLog(cfg RequestLogConfig, add func(string, ...any)) {
	if cfg.RetentionDays < 0 {
		add("request-log.retention-days cannot be negative")
	}
	if cfg.MaxBodyBytes < 0 {
		add("request-log.max-body-bytes cannot be negative")
	}
	switch cfg.BodyMode {
	case "", BodyModeFailedOnly, BodyModeAlways, BodyModeNone:
	default:
		add("request-log.body-mode must be one of %s, %s, %s", BodyModeFailedOnly, BodyModeAlways, BodyModeNone)
	}
}

func validateScheduler(cfg SchedulerConfig, add func(string, ...any)) {
	if cfg.MaxRetries < 0 {
		add("scheduler.max-retries cannot be negative")
	}
	if cfg.CircuitCooldownSec < 0 {
		add("scheduler.circuit-cooldown-sec cannot be negative")
	}
	if cfg.CircuitFailureThreshold < 0 {
		add("scheduler.circuit-failure-threshold cannot be negative")
	}
}

func validateCORS(cfg CORSConfig, add func(string, ...any)) {
	for i, origin := range cfg.AllowedOrigins {
		if strings.TrimSpace(origin) == "" {
			add("cors.allowed-origins[%d] cannot be empty", i)
		}
	}
}

func validateHTTPURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("is invalid: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" && parsed.Scheme != "socks5" {
		return fmt.Errorf("must use http, https, or socks5")
	}
	if parsed.Host == "" {
		return fmt.Errorf("must include host")
	}
	return nil
}
