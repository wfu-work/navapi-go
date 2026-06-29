package services

import "strconv"

type RiskControlSettings struct {
	MaxBodyBytes                int64  `json:"maxBodyBytes"`
	ModelRateLimitCount         int64  `json:"modelRateLimitCount"`
	ModelRateLimitWindowSeconds int64  `json:"modelRateLimitWindowSeconds"`
	ChannelAffinitySeconds      int64  `json:"channelAffinitySeconds"`
	SSRFCheckEnabled            bool   `json:"ssrfCheckEnabled"`
	SensitiveWords              string `json:"sensitiveWords"`
}

type RiskControlService struct{}

var RiskControlServiceApp = RiskControlService{}

const defaultRiskMaxBodyBytes int64 = 32 << 20

func (s RiskControlService) Get() RiskControlSettings {
	return RiskControlSettings{
		MaxBodyBytes:                OptionServiceApp.Int64("relay.max_body_bytes", defaultRiskMaxBodyBytes),
		ModelRateLimitCount:         OptionServiceApp.Int64("relay.model_rate_limit_count", 0),
		ModelRateLimitWindowSeconds: OptionServiceApp.Int64("relay.model_rate_limit_window_seconds", 60),
		ChannelAffinitySeconds:      OptionServiceApp.Int64("relay.channel_affinity_seconds", 0),
		SSRFCheckEnabled:            OptionServiceApp.Int64("relay.ssrf_check_enabled", 1) > 0,
		SensitiveWords:              OptionServiceApp.Get("relay.sensitive_words", ""),
	}
}

// Set writes all relay risk-control knobs through OptionService so the in-memory
// option cache stays in sync with persisted values.
func (s RiskControlService) Set(settings RiskControlSettings) error {
	values := map[string]string{
		"relay.max_body_bytes":                  strconv.FormatInt(settings.MaxBodyBytes, 10),
		"relay.model_rate_limit_count":          strconv.FormatInt(settings.ModelRateLimitCount, 10),
		"relay.model_rate_limit_window_seconds": strconv.FormatInt(settings.ModelRateLimitWindowSeconds, 10),
		"relay.channel_affinity_seconds":        strconv.FormatInt(settings.ChannelAffinitySeconds, 10),
		"relay.sensitive_words":                 settings.SensitiveWords,
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
