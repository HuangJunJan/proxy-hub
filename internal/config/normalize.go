package config

func Normalize(cfg *Config) {
	if cfg == nil {
		return
	}
	if cfg.Server.Host == DefaultHost {
		cfg.Server.Host = ""
	}
	if cfg.Server.Port == DefaultPort {
		cfg.Server.Port = 0
	}
	if cfg.Server.OpenBrowser != nil && *cfg.Server.OpenBrowser == DefaultOpenBrowser {
		cfg.Server.OpenBrowser = nil
	}
	if cfg.RequestLog.RetentionDays == DefaultRetentionDays {
		cfg.RequestLog.RetentionDays = 0
	}
	if cfg.RequestLog.BodyMode == DefaultRequestBodyMode {
		cfg.RequestLog.BodyMode = ""
	}
	if cfg.RequestLog.MaxBodyBytes == DefaultMaxBodyBytes {
		cfg.RequestLog.MaxBodyBytes = 0
	}
	if cfg.Scheduler.MaxRetries == DefaultMaxRetries {
		cfg.Scheduler.MaxRetries = 0
	}
	if cfg.Scheduler.CircuitCooldownSec == DefaultCircuitCooldownSec {
		cfg.Scheduler.CircuitCooldownSec = 0
	}
	if cfg.Scheduler.CircuitFailureThreshold == DefaultCircuitFailureThreshold {
		cfg.Scheduler.CircuitFailureThreshold = 0
	}
	for i := range cfg.OpenAIAPI {
		if cfg.OpenAIAPI[i].Priority == DefaultPriority {
			cfg.OpenAIAPI[i].Priority = 0
		}
		if cfg.OpenAIAPI[i].TimeoutSec == DefaultTimeoutSec {
			cfg.OpenAIAPI[i].TimeoutSec = 0
		}
	}
	for i := range cfg.ChatGPTOAuth {
		if cfg.ChatGPTOAuth[i].TimeoutSec == DefaultTimeoutSec {
			cfg.ChatGPTOAuth[i].TimeoutSec = 0
		}
	}
}
