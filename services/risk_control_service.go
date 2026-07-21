package services

import "strconv"

type RiskControlSettings struct {
	MaxBodyBytes                int64  `json:"maxBodyBytes"`
	MaxUpstreamResponseBytes    int64  `json:"maxUpstreamResponseBytes"`
	ModelRateLimitCount         int64  `json:"modelRateLimitCount"`
	ModelRateLimitWindowSeconds int64  `json:"modelRateLimitWindowSeconds"`
	ProviderAffinitySeconds     int64  `json:"providerAffinitySeconds"`
	ProviderCircuitEnabled      bool   `json:"providerCircuitEnabled"`
	ProviderFailureThreshold    int64  `json:"providerFailureThreshold"`
	ProviderCooldownSeconds     int64  `json:"providerCooldownSeconds"`
	ProviderMaxCooldownSeconds  int64  `json:"providerMaxCooldownSeconds"`
	SSRFCheckEnabled            bool   `json:"ssrfCheckEnabled"`
	SensitiveWords              string `json:"sensitiveWords"`
}

type RiskControlService struct{}

var RiskControlServiceApp = new(RiskControlService)

const (
	defaultRiskMaxBodyBytes             int64 = 32 << 20
	defaultRiskMaxUpstreamResponseBytes int64 = 64 << 20
)

func (s RiskControlService) Get() RiskControlSettings {
	return RiskControlSettings{
		MaxBodyBytes:                OptionServiceApp.Int64("relay.max_body_bytes", defaultRiskMaxBodyBytes),
		MaxUpstreamResponseBytes:    OptionServiceApp.Int64("relay.max_upstream_response_bytes", defaultRiskMaxUpstreamResponseBytes),
		ModelRateLimitCount:         OptionServiceApp.Int64("relay.model_rate_limit_count", 0),
		ModelRateLimitWindowSeconds: OptionServiceApp.Int64("relay.model_rate_limit_window_seconds", 60),
		ProviderAffinitySeconds:     OptionServiceApp.Int64("relay.provider_affinity_seconds", 0),
		ProviderCircuitEnabled:      OptionServiceApp.Int64("relay.provider_circuit_enabled", 1) > 0,
		ProviderFailureThreshold:    OptionServiceApp.Int64("relay.provider_failure_threshold", 2),
		ProviderCooldownSeconds:     OptionServiceApp.Int64("relay.provider_cooldown_seconds", 30),
		ProviderMaxCooldownSeconds:  OptionServiceApp.Int64("relay.provider_max_cooldown_seconds", 600),
		SSRFCheckEnabled:            OptionServiceApp.Int64("relay.ssrf_check_enabled", 1) > 0,
		SensitiveWords:              OptionServiceApp.Get("relay.sensitive_words", ""),
	}
}

// Set writes all relay risk-control knobs through OptionService so the in-memory
// option cache stays in sync with persisted values.
func (s RiskControlService) Set(settings RiskControlSettings) error {
	if settings.MaxBodyBytes < 0 {
		settings.MaxBodyBytes = 0
	}
	if settings.MaxUpstreamResponseBytes < 0 {
		settings.MaxUpstreamResponseBytes = 0
	}
	if settings.ModelRateLimitCount < 0 {
		settings.ModelRateLimitCount = 0
	}
	if settings.ModelRateLimitWindowSeconds <= 0 {
		settings.ModelRateLimitWindowSeconds = 60
	}
	if settings.ProviderAffinitySeconds < 0 {
		settings.ProviderAffinitySeconds = 0
	}
	if settings.ProviderFailureThreshold <= 0 {
		settings.ProviderFailureThreshold = 2
	}
	if settings.ProviderCooldownSeconds <= 0 {
		settings.ProviderCooldownSeconds = 30
	}
	if settings.ProviderMaxCooldownSeconds < settings.ProviderCooldownSeconds {
		settings.ProviderMaxCooldownSeconds = settings.ProviderCooldownSeconds
	}
	values := map[string]string{
		"relay.max_body_bytes":                  strconv.FormatInt(settings.MaxBodyBytes, 10),
		"relay.max_upstream_response_bytes":     strconv.FormatInt(settings.MaxUpstreamResponseBytes, 10),
		"relay.model_rate_limit_count":          strconv.FormatInt(settings.ModelRateLimitCount, 10),
		"relay.model_rate_limit_window_seconds": strconv.FormatInt(settings.ModelRateLimitWindowSeconds, 10),
		"relay.provider_affinity_seconds":       strconv.FormatInt(settings.ProviderAffinitySeconds, 10),
		"relay.provider_failure_threshold":      strconv.FormatInt(settings.ProviderFailureThreshold, 10),
		"relay.provider_cooldown_seconds":       strconv.FormatInt(settings.ProviderCooldownSeconds, 10),
		"relay.provider_max_cooldown_seconds":   strconv.FormatInt(settings.ProviderMaxCooldownSeconds, 10),
		"relay.sensitive_words":                 settings.SensitiveWords,
	}
	if settings.ProviderCircuitEnabled {
		values["relay.provider_circuit_enabled"] = "1"
	} else {
		values["relay.provider_circuit_enabled"] = "0"
	}
	if settings.SSRFCheckEnabled {
		values["relay.ssrf_check_enabled"] = "1"
	} else {
		values["relay.ssrf_check_enabled"] = "0"
	}
	for key, value := range values {
		if err := OptionServiceApp.Set(key, value); err != nil {
			return err
		}
	}
	return nil
}
