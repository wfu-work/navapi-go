package services

import (
	"strconv"
	"strings"

	"navapi-go/constants"
	"navapi-go/domains"
)

const (
	settingRegisterEnabled        = "register.enabled"
	settingRegisterDefaultAmount  = "register.default_amount"
	settingRegisterDefaultGroup   = "register.default_group"
	settingRegisterAllowedGroups  = "register.allowed_groups"
	settingRegisterRequireInvite  = "register.require_invite"
	settingRegisterRequireCaptcha = "register.require_captcha"
	settingRegisterNotice         = "register.notice"

	defaultRegisterAmount = int64(10)
	defaultRegisterNotice = "注册成功后将创建普通用户账号并发放默认金额。请使用真实邮箱接收验证码；如当前要求邀请码，请联系管理员获取。"

	registerDisabledMessage = "注册入口已关闭，请联系管理员"
)

type RegisterSettings struct {
	Enabled        bool   `json:"enabled"`
	DefaultAmount  int64  `json:"defaultAmount"`
	DefaultGroup   string `json:"defaultGroup"`
	AllowedGroups  string `json:"allowedGroups"`
	RequireInvite  bool   `json:"requireInvite"`
	RequireCaptcha bool   `json:"requireCaptcha"`
	Notice         string `json:"notice"`
}

type RegisterSettingService struct{}

var RegisterSettingServiceApp = RegisterSettingService{}

func (s RegisterSettingService) Get() RegisterSettings {
	settings := defaultRegisterSettings()
	values := s.values()
	settings.Enabled = settingBool(values[settingRegisterEnabled], settings.Enabled)
	settings.DefaultAmount = settingInt64(values[settingRegisterDefaultAmount], settings.DefaultAmount)
	settings.DefaultGroup = settingText(values[settingRegisterDefaultGroup], settings.DefaultGroup)
	settings.AllowedGroups = settingText(values[settingRegisterAllowedGroups], settings.AllowedGroups)
	settings.RequireInvite = settingBool(values[settingRegisterRequireInvite], settings.RequireInvite)
	settings.RequireCaptcha = settingBool(values[settingRegisterRequireCaptcha], settings.RequireCaptcha)
	settings.Notice = settingText(values[settingRegisterNotice], settings.Notice)
	return normalizeRegisterSettings(settings)
}

func defaultRegisterSettings() RegisterSettings {
	return RegisterSettings{
		Enabled:        true,
		DefaultAmount:  defaultRegisterAmount,
		DefaultGroup:   constants.DefaultGroup,
		AllowedGroups:  "",
		RequireInvite:  false,
		RequireCaptcha: true,
		Notice:         defaultRegisterNotice,
	}
}

// Set stores registration knobs in Setting so admin pages and registration flow
// consume a single source of truth.
func (s RegisterSettingService) Set(settings RegisterSettings) error {
	settings = normalizeRegisterSettings(settings)
	items := []domains.Setting{
		{Key: settingRegisterEnabled, Value: boolSetting(settings.Enabled), Description: "注册设置：是否开放注册"},
		{Key: settingRegisterDefaultAmount, Value: strconv.FormatInt(settings.DefaultAmount, 10), Description: "注册设置：默认金额"},
		{Key: settingRegisterDefaultGroup, Value: settings.DefaultGroup, Description: "注册设置：默认模型分组"},
		{Key: settingRegisterAllowedGroups, Value: settings.AllowedGroups, Description: "注册设置：可用模型分组"},
		{Key: settingRegisterRequireInvite, Value: boolSetting(settings.RequireInvite), Description: "注册设置：是否必须邀请码"},
		{Key: settingRegisterRequireCaptcha, Value: boolSetting(settings.RequireCaptcha), Description: "注册设置：是否必须验证码"},
		{Key: settingRegisterNotice, Value: settings.Notice, Description: "注册设置：注册页提示"},
	}
	for _, item := range items {
		if err := SettingServiceApp.Save(item); err != nil {
			return err
		}
	}
	return nil
}

func (s RegisterSettingService) values() map[string]string {
	db := SettingServiceApp.DB()
	if db == nil || !db.Migrator().HasTable(&domains.Setting{}) {
		return map[string]string{}
	}
	keys := []string{
		settingRegisterEnabled,
		settingRegisterDefaultAmount,
		settingRegisterDefaultGroup,
		settingRegisterAllowedGroups,
		settingRegisterRequireInvite,
		settingRegisterRequireCaptcha,
		settingRegisterNotice,
	}
	var rows []domains.Setting
	if err := db.Where("key IN ?", keys).Find(&rows).Error; err != nil {
		return map[string]string{}
	}
	values := map[string]string{}
	for _, row := range rows {
		values[row.Key] = row.Value
	}
	return values
}

func normalizeRegisterSettings(settings RegisterSettings) RegisterSettings {
	settings.DefaultGroup = strings.TrimSpace(settings.DefaultGroup)
	if settings.DefaultGroup == "" {
		settings.DefaultGroup = constants.DefaultGroup
	}
	if settings.DefaultAmount < 0 {
		settings.DefaultAmount = 0
	}
	settings.AllowedGroups = normalizeRegisterGroups(settings.AllowedGroups)
	settings.Notice = strings.TrimSpace(settings.Notice)
	return settings
}

func normalizeRegisterGroups(value string) string {
	parts := strings.Split(value, ",")
	groups := make([]string, 0, len(parts))
	for _, part := range parts {
		group := strings.TrimSpace(part)
		if group != "" {
			groups = append(groups, group)
		}
	}
	return strings.Join(groups, ",")
}

func settingText(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func settingInt64(value string, fallback int64) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func settingBool(value string, fallback bool) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func boolSetting(value bool) string {
	if value {
		return "1"
	}
	return "0"
}
