package config

import (
	"net/url"
	"strings"
	"time"
)

const (
	DefaultHost                    = "0.0.0.0"
	DefaultPort                    = 8787
	DefaultOpenBrowser             = true
	DefaultPriority                = 100
	DefaultTimeoutSec              = 120
	DefaultRetentionDays           = 7
	DefaultRequestBodyMode         = BodyModeFailedOnly
	DefaultMaxBodyBytes            = 65536
	DefaultMaxRetries              = 2
	DefaultCircuitCooldownSec      = 60
	DefaultCircuitFailureThreshold = 3

	ChannelTypeOpenAIAPI    = "openai-api"
	ChannelTypeChatGPTOAuth = "chatgpt-oauth"

	BodyModeFailedOnly = "failed_only"
	BodyModeAlways     = "always"
	BodyModeNone       = "none"
)

type Config struct {
	Server       ServerConfig          `yaml:"server,omitempty" json:"server,omitempty"`
	Admin        *AdminConfig          `yaml:"admin,omitempty" json:"admin,omitempty"`
	APIKeys      []APIKeyConfig        `yaml:"api-keys,omitempty" json:"api-keys,omitempty"`
	OpenAIAPI    []OpenAIAPIChannel    `yaml:"openai-api,omitempty" json:"openai-api,omitempty"`
	ChatGPTOAuth []ChatGPTOAuthChannel `yaml:"chatgpt-oauth,omitempty" json:"chatgpt-oauth,omitempty"`
	RequestLog   RequestLogConfig      `yaml:"request-log,omitempty" json:"request-log,omitempty"`
	Scheduler    SchedulerConfig       `yaml:"scheduler,omitempty" json:"scheduler,omitempty"`
	CORS         CORSConfig            `yaml:"cors,omitempty" json:"cors,omitempty"`
}

type ServerConfig struct {
	Host        string `yaml:"host,omitempty" json:"host,omitempty"`
	Port        int    `yaml:"port,omitempty" json:"port,omitempty"`
	OpenBrowser *bool  `yaml:"open-browser,omitempty" json:"open-browser,omitempty"`
}

type AdminConfig struct {
	Username     string `yaml:"username,omitempty" json:"username,omitempty"`
	PasswordHash string `yaml:"password-hash,omitempty" json:"password-hash,omitempty"`
}

type APIKeyConfig struct {
	Token    string `yaml:"token,omitempty" json:"token,omitempty"`
	Name     string `yaml:"name,omitempty" json:"name,omitempty"`
	Notes    string `yaml:"notes,omitempty" json:"notes,omitempty"`
	Disabled bool   `yaml:"disabled,omitempty" json:"disabled,omitempty"`
}

type OpenAIAPIChannel struct {
	Name          string        `yaml:"name,omitempty" json:"name,omitempty"`
	BaseURL       string        `yaml:"base-url,omitempty" json:"base-url,omitempty"`
	Priority      int           `yaml:"priority,omitempty" json:"priority,omitempty"`
	APIKeyEntries []APIKeyEntry `yaml:"api-key-entries,omitempty" json:"api-key-entries,omitempty"`
	Models        []ModelEntry  `yaml:"models,omitempty" json:"models,omitempty"`
	Disabled      bool          `yaml:"disabled,omitempty" json:"disabled,omitempty"`
	TimeoutSec    int           `yaml:"timeout-sec,omitempty" json:"timeout-sec,omitempty"`
	Notes         string        `yaml:"notes,omitempty" json:"notes,omitempty"`
}

type APIKeyEntry struct {
	APIKey   string `yaml:"api-key,omitempty" json:"api-key,omitempty"`
	ProxyURL string `yaml:"proxy-url,omitempty" json:"proxy-url,omitempty"`
}

type ChatGPTOAuthChannel struct {
	Name       string       `yaml:"name,omitempty" json:"name,omitempty"`
	OAuth      OAuthConfig  `yaml:"oauth,omitempty" json:"oauth,omitempty"`
	Models     []ModelEntry `yaml:"models,omitempty" json:"models,omitempty"`
	Disabled   bool         `yaml:"disabled,omitempty" json:"disabled,omitempty"`
	TimeoutSec int          `yaml:"timeout-sec,omitempty" json:"timeout-sec,omitempty"`
	Notes      string       `yaml:"notes,omitempty" json:"notes,omitempty"`
}

type OAuthConfig struct {
	AccessToken  string    `yaml:"access-token,omitempty" json:"access-token,omitempty"`
	RefreshToken string    `yaml:"refresh-token,omitempty" json:"refresh-token,omitempty"`
	ExpiresAt    time.Time `yaml:"expires-at,omitempty" json:"expires-at,omitempty"`
}

type ModelEntry struct {
	Name  string `yaml:"name,omitempty" json:"name,omitempty"`
	Alias string `yaml:"alias,omitempty" json:"alias,omitempty"`
}

type RequestLogConfig struct {
	RetentionDays int    `yaml:"retention-days,omitempty" json:"retention-days,omitempty"`
	BodyMode      string `yaml:"body-mode,omitempty" json:"body-mode,omitempty"`
	MaxBodyBytes  int    `yaml:"max-body-bytes,omitempty" json:"max-body-bytes,omitempty"`
}

type SchedulerConfig struct {
	MaxRetries              int `yaml:"max-retries,omitempty" json:"max-retries,omitempty"`
	CircuitCooldownSec      int `yaml:"circuit-cooldown-sec,omitempty" json:"circuit-cooldown-sec,omitempty"`
	CircuitFailureThreshold int `yaml:"circuit-failure-threshold,omitempty" json:"circuit-failure-threshold,omitempty"`
}

type CORSConfig struct {
	AllowedOrigins []string `yaml:"allowed-origins,omitempty" json:"allowed-origins,omitempty"`
}

func (c Config) EffectiveServerHost() string {
	if strings.TrimSpace(c.Server.Host) == "" {
		return DefaultHost
	}
	return strings.TrimSpace(c.Server.Host)
}

func (c Config) EffectiveServerPort() int {
	if c.Server.Port == 0 {
		return DefaultPort
	}
	return c.Server.Port
}

func (c Config) EffectiveOpenBrowser() bool {
	if c.Server.OpenBrowser == nil {
		return DefaultOpenBrowser
	}
	return *c.Server.OpenBrowser
}

func (c Config) EffectiveRequestLogRetentionDays() int {
	if c.RequestLog.RetentionDays == 0 {
		return DefaultRetentionDays
	}
	return c.RequestLog.RetentionDays
}

func (c Config) EffectiveRequestLogBodyMode() string {
	if c.RequestLog.BodyMode == "" {
		return DefaultRequestBodyMode
	}
	return c.RequestLog.BodyMode
}

func (c Config) EffectiveRequestLogMaxBodyBytes() int {
	if c.RequestLog.MaxBodyBytes == 0 {
		return DefaultMaxBodyBytes
	}
	return c.RequestLog.MaxBodyBytes
}

func (c Config) EffectiveMaxRetries() int {
	if c.Scheduler.MaxRetries == 0 {
		return DefaultMaxRetries
	}
	return c.Scheduler.MaxRetries
}

func (c Config) EffectiveCircuitCooldownSec() int {
	if c.Scheduler.CircuitCooldownSec == 0 {
		return DefaultCircuitCooldownSec
	}
	return c.Scheduler.CircuitCooldownSec
}

func (c Config) EffectiveCircuitFailureThreshold() int {
	if c.Scheduler.CircuitFailureThreshold == 0 {
		return DefaultCircuitFailureThreshold
	}
	return c.Scheduler.CircuitFailureThreshold
}

func (c OpenAIAPIChannel) EffectivePriority() int {
	if c.Priority == 0 {
		return DefaultPriority
	}
	return c.Priority
}

func (c OpenAIAPIChannel) EffectiveTimeoutSec() int {
	if c.TimeoutSec == 0 {
		return DefaultTimeoutSec
	}
	return c.TimeoutSec
}

func (c ChatGPTOAuthChannel) EffectiveTimeoutSec() int {
	if c.TimeoutSec == 0 {
		return DefaultTimeoutSec
	}
	return c.TimeoutSec
}

func (m ModelEntry) EffectiveAlias() string {
	if strings.TrimSpace(m.Alias) != "" {
		return strings.TrimSpace(m.Alias)
	}
	return strings.TrimSpace(m.Name)
}

func (e APIKeyEntry) HasProxyURL() bool {
	return strings.TrimSpace(e.ProxyURL) != ""
}

func (e APIKeyEntry) ParseProxyURL() (*url.URL, error) {
	if !e.HasProxyURL() {
		return nil, nil
	}
	return url.Parse(strings.TrimSpace(e.ProxyURL))
}
