package services

import "strconv"

type RegisterSettings struct {
	Enabled        bool   `json:"enabled"`
	DefaultQuota   int64  `json:"defaultQuota"`
	DefaultGroup   string `json:"defaultGroup"`
	AllowedGroups  string `json:"allowedGroups"`
	RequireInvite  bool   `json:"requireInvite"`
	RequireCaptcha bool   `json:"requireCaptcha"`
	Notice         string `json:"notice"`
}

type RegisterSettingService struct{}

var RegisterSettingServiceApp = RegisterSettingService{}

func (s RegisterSettingService) Get() RegisterSettings {
	return RegisterSettings{
		Enabled:        OptionServiceApp.Int64("register.enabled", 1) > 0,
		DefaultQuota:   OptionServiceApp.Int64("register.default_quota", 0),
		DefaultGroup:   OptionServiceApp.Get("register.default_group", "default"),
		AllowedGroups:  OptionServiceApp.Get("register.allowed_groups", ""),
		RequireInvite:  OptionServiceApp.Int64("register.require_invite", 0) > 0,
		RequireCaptcha: OptionServiceApp.Int64("register.require_captcha", 0) > 0,
		Notice:         OptionServiceApp.Get("register.notice", ""),
	}
}

// Set stores registration knobs in OptionService so common registration code or
// future hooks can consume a single source of truth.
func (s RegisterSettingService) Set(settings RegisterSettings) error {
	values := map[string]string{
		"register.default_quota":  strconv.FormatInt(settings.DefaultQuota, 10),
		"register.default_group":  settings.DefaultGroup,
		"register.allowed_groups": settings.AllowedGroups,
		"register.notice":         settings.Notice,
	}
	if settings.Enabled {
		values["register.enabled"] = "1"
	} else {
		values["register.enabled"] = "0"
	}
	if settings.RequireInvite {
		values["register.require_invite"] = "1"
	} else {
		values["register.require_invite"] = "0"
	}
	if settings.RequireCaptcha {
		values["register.require_captcha"] = "1"
	} else {
		values["register.require_captcha"] = "0"
	}
	for key, value := range values {
		if err := OptionServiceApp.Set(key, value); err != nil {
			return err
		}
	}
	return nil
}
