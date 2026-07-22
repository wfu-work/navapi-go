package services

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type RiskControlSettings struct {
	MaxBodyBytes                      int64  `json:"maxBodyBytes"`
	MaxUpstreamResponseBytes          int64  `json:"maxUpstreamResponseBytes"`
	ModelRateLimitEnabled             bool   `json:"modelRateLimitEnabled"`
	ModelRateLimitCount               int64  `json:"modelRateLimitCount"`
	ModelRateLimitWindowSeconds       int64  `json:"modelRateLimitWindowSeconds"`
	ProviderAffinityEnabled           bool   `json:"providerAffinityEnabled"`
	ProviderAffinitySeconds           int64  `json:"providerAffinitySeconds"`
	ProviderCircuitEnabled            bool   `json:"providerCircuitEnabled"`
	ProviderFailureThreshold          int64  `json:"providerFailureThreshold"`
	ProviderCooldownSeconds           int64  `json:"providerCooldownSeconds"`
	ProviderMaxCooldownSeconds        int64  `json:"providerMaxCooldownSeconds"`
	ResponsesSynthesizeCompletedOnEOF bool   `json:"responsesSynthesizeCompletedOnEOF"`
	ResponsesEOFTerminalPolicy        string `json:"responsesEOFTerminalPolicy"`
	SSRFCheckEnabled                  bool   `json:"ssrfCheckEnabled"`
	SensitiveWords                    string `json:"sensitiveWords"`
}

type RiskControlService struct{}

var RiskControlServiceApp = new(RiskControlService)

const (
	defaultRiskMaxBodyBytes                int64 = 32 << 20
	defaultRiskMaxUpstreamResponseBytes    int64 = 64 << 20
	defaultRiskModelRateLimitCount         int64 = 60
	defaultRiskModelRateLimitWindowSeconds int64 = 60
	defaultRiskProviderAffinitySeconds     int64 = 300
	defaultRiskProviderFailureThreshold    int64 = 2
	defaultRiskProviderCooldownSeconds     int64 = 30
	defaultRiskProviderMaxCooldownSeconds  int64 = 600
	maximumRiskBodyBytes                   int64 = 1 << 30
	maximumRiskUpstreamResponseBytes       int64 = 2 << 30
	maximumRiskModelRateLimitCount         int64 = 1_000_000
	maximumRiskModelRateLimitWindowSeconds int64 = 24 * 60 * 60
	maximumRiskProviderAffinitySeconds     int64 = 24 * 60 * 60
	maximumRiskProviderFailureThreshold    int64 = 100
	maximumRiskProviderCooldownSeconds     int64 = 24 * 60 * 60
	maximumRiskProviderMaxCooldownSeconds  int64 = 7 * 24 * 60 * 60
	maximumRiskSensitiveWordsBytes               = 64 << 10
)

func (s RiskControlService) Get() RiskControlSettings {
	storedRateLimitCount := OptionServiceApp.Int64("relay.model_rate_limit_count", 0)
	storedAffinitySeconds := OptionServiceApp.Int64("relay.provider_affinity_seconds", 0)
	rateLimitCount := storedRateLimitCount
	if rateLimitCount <= 0 {
		rateLimitCount = defaultRiskModelRateLimitCount
	}
	affinitySeconds := storedAffinitySeconds
	if affinitySeconds <= 0 {
		affinitySeconds = defaultRiskProviderAffinitySeconds
	}
	responsesEOFTerminalPolicy := responsesStreamEOFTerminalPolicy()
	return RiskControlSettings{
		MaxBodyBytes:                      OptionServiceApp.Int64("relay.max_body_bytes", defaultRiskMaxBodyBytes),
		MaxUpstreamResponseBytes:          OptionServiceApp.Int64("relay.max_upstream_response_bytes", defaultRiskMaxUpstreamResponseBytes),
		ModelRateLimitEnabled:             OptionServiceApp.Bool("relay.model_rate_limit_enabled", storedRateLimitCount > 0),
		ModelRateLimitCount:               rateLimitCount,
		ModelRateLimitWindowSeconds:       OptionServiceApp.Int64("relay.model_rate_limit_window_seconds", defaultRiskModelRateLimitWindowSeconds),
		ProviderAffinityEnabled:           OptionServiceApp.Bool("relay.provider_affinity_enabled", storedAffinitySeconds > 0),
		ProviderAffinitySeconds:           affinitySeconds,
		ProviderCircuitEnabled:            OptionServiceApp.Bool("relay.provider_circuit_enabled", true),
		ProviderFailureThreshold:          OptionServiceApp.Int64("relay.provider_failure_threshold", defaultRiskProviderFailureThreshold),
		ProviderCooldownSeconds:           OptionServiceApp.Int64("relay.provider_cooldown_seconds", defaultRiskProviderCooldownSeconds),
		ProviderMaxCooldownSeconds:        OptionServiceApp.Int64("relay.provider_max_cooldown_seconds", defaultRiskProviderMaxCooldownSeconds),
		ResponsesSynthesizeCompletedOnEOF: responsesEOFTerminalPolicy != responsesEOFTerminalPolicyOff,
		ResponsesEOFTerminalPolicy:        responsesEOFTerminalPolicy,
		SSRFCheckEnabled:                  OptionServiceApp.Bool("relay.ssrf_check_enabled", true),
		SensitiveWords:                    OptionServiceApp.Get("relay.sensitive_words", ""),
	}
}

func (s RiskControlService) Set(settings RiskControlSettings) error {
	if len(settings.SensitiveWords) > maximumRiskSensitiveWordsBytes {
		return fmt.Errorf("sensitiveWords must not exceed %d bytes", maximumRiskSensitiveWordsBytes)
	}
	settings.SensitiveWords = normalizeRiskSensitiveWords(settings.SensitiveWords)
	if err := validateRiskControlSettings(&settings); err != nil {
		return err
	}
	values := map[string]string{
		"relay.max_body_bytes":                        strconv.FormatInt(settings.MaxBodyBytes, 10),
		"relay.max_upstream_response_bytes":           strconv.FormatInt(settings.MaxUpstreamResponseBytes, 10),
		"relay.model_rate_limit_enabled":              strconv.FormatBool(settings.ModelRateLimitEnabled),
		"relay.model_rate_limit_count":                strconv.FormatInt(settings.ModelRateLimitCount, 10),
		"relay.model_rate_limit_window_seconds":       strconv.FormatInt(settings.ModelRateLimitWindowSeconds, 10),
		"relay.provider_affinity_enabled":             strconv.FormatBool(settings.ProviderAffinityEnabled),
		"relay.provider_affinity_seconds":             strconv.FormatInt(settings.ProviderAffinitySeconds, 10),
		"relay.provider_circuit_enabled":              strconv.FormatBool(settings.ProviderCircuitEnabled),
		"relay.provider_failure_threshold":            strconv.FormatInt(settings.ProviderFailureThreshold, 10),
		"relay.provider_cooldown_seconds":             strconv.FormatInt(settings.ProviderCooldownSeconds, 10),
		"relay.provider_max_cooldown_seconds":         strconv.FormatInt(settings.ProviderMaxCooldownSeconds, 10),
		"relay.responses_synthesize_completed_on_eof": strconv.FormatBool(settings.ResponsesSynthesizeCompletedOnEOF),
		"relay.responses_eof_terminal_policy":         settings.ResponsesEOFTerminalPolicy,
		"relay.ssrf_check_enabled":                    strconv.FormatBool(settings.SSRFCheckEnabled),
		"relay.sensitive_words":                       settings.SensitiveWords,
	}
	if err := OptionServiceApp.SetMany(values); err != nil {
		return err
	}
	RateLimitServiceApp.Reset()
	ProviderServiceApp.ResetAffinity()
	ProviderCircuitBreakerApp.Reset()
	return nil
}

func validateRiskControlSettings(settings *RiskControlSettings) error {
	if settings == nil {
		return errors.New("risk control settings are required")
	}
	if settings.MaxBodyBytes < 0 || settings.MaxBodyBytes > maximumRiskBodyBytes {
		return fmt.Errorf("maxBodyBytes must be between 0 and %d", maximumRiskBodyBytes)
	}
	if settings.MaxUpstreamResponseBytes < 0 || settings.MaxUpstreamResponseBytes > maximumRiskUpstreamResponseBytes {
		return fmt.Errorf("maxUpstreamResponseBytes must be between 0 and %d", maximumRiskUpstreamResponseBytes)
	}
	if settings.ModelRateLimitCount <= 0 {
		if settings.ModelRateLimitEnabled {
			return errors.New("modelRateLimitCount must be greater than 0 when model rate limiting is enabled")
		}
		settings.ModelRateLimitCount = defaultRiskModelRateLimitCount
	}
	if settings.ModelRateLimitCount > maximumRiskModelRateLimitCount {
		return fmt.Errorf("modelRateLimitCount must be less than or equal to %d", maximumRiskModelRateLimitCount)
	}
	if settings.ModelRateLimitWindowSeconds <= 0 || settings.ModelRateLimitWindowSeconds > maximumRiskModelRateLimitWindowSeconds {
		return fmt.Errorf("modelRateLimitWindowSeconds must be between 1 and %d", maximumRiskModelRateLimitWindowSeconds)
	}
	if settings.ProviderAffinitySeconds <= 0 {
		if settings.ProviderAffinityEnabled {
			return errors.New("providerAffinitySeconds must be greater than 0 when provider affinity is enabled")
		}
		settings.ProviderAffinitySeconds = defaultRiskProviderAffinitySeconds
	}
	if settings.ProviderAffinitySeconds > maximumRiskProviderAffinitySeconds {
		return fmt.Errorf("providerAffinitySeconds must be less than or equal to %d", maximumRiskProviderAffinitySeconds)
	}
	if settings.ProviderFailureThreshold <= 0 || settings.ProviderFailureThreshold > maximumRiskProviderFailureThreshold {
		return fmt.Errorf("providerFailureThreshold must be between 1 and %d", maximumRiskProviderFailureThreshold)
	}
	if settings.ProviderCooldownSeconds <= 0 || settings.ProviderCooldownSeconds > maximumRiskProviderCooldownSeconds {
		return fmt.Errorf("providerCooldownSeconds must be between 1 and %d", maximumRiskProviderCooldownSeconds)
	}
	if settings.ProviderMaxCooldownSeconds < settings.ProviderCooldownSeconds || settings.ProviderMaxCooldownSeconds > maximumRiskProviderMaxCooldownSeconds {
		return fmt.Errorf("providerMaxCooldownSeconds must be between providerCooldownSeconds and %d", maximumRiskProviderMaxCooldownSeconds)
	}
	responsesEOFTerminalPolicy, err := normalizeRiskResponsesEOFTerminalPolicy(settings.ResponsesEOFTerminalPolicy, settings.ResponsesSynthesizeCompletedOnEOF)
	if err != nil {
		return err
	}
	settings.ResponsesEOFTerminalPolicy = responsesEOFTerminalPolicy
	settings.ResponsesSynthesizeCompletedOnEOF = responsesEOFTerminalPolicy != responsesEOFTerminalPolicyOff
	if len(settings.SensitiveWords) > maximumRiskSensitiveWordsBytes {
		return fmt.Errorf("sensitiveWords must not exceed %d bytes", maximumRiskSensitiveWordsBytes)
	}
	return nil
}

func normalizeRiskResponsesEOFTerminalPolicy(value string, legacyEnabled bool) (string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		if legacyEnabled {
			return responsesEOFTerminalPolicyIncomplete, nil
		}
		return responsesEOFTerminalPolicyOff, nil
	}
	if policy, ok := normalizeResponsesEOFTerminalPolicy(raw); ok {
		return policy, nil
	}
	return "", errors.New("responsesEOFTerminalPolicy must be one of completed, incomplete, failed or off")
}

func normalizeRiskSensitiveWords(raw string) string {
	words := splitCSV(raw)
	seen := make(map[string]struct{}, len(words))
	unique := make([]string, 0, len(words))
	for _, word := range words {
		key := strings.ToLower(word)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, word)
	}
	return strings.Join(unique, "\n")
}
