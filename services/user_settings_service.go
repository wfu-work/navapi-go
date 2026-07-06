package services

import (
	"errors"
	"strings"

	"navapi-go/domains"

	commonServices "github.com/wfu-work/nav-common-go-lib/services"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const DefaultUserMaxConcurrency = 5

type UserSettingsService struct {
	commonServices.CrudService[domains.UserSettings]
}

var UserSettingsServiceApp = new(UserSettingsService)

func (s *UserSettingsService) WithDB(db *gorm.DB) *UserSettingsService {
	cloned := *s
	cloned.CrudService = *s.CrudService.WithDB(db)
	return &cloned
}

func (s *UserSettingsService) Ensure(tx *gorm.DB, userGuid string) error {
	userGuid = strings.TrimSpace(userGuid)
	if userGuid == "" {
		return nil
	}
	settings := defaultUserSettings(userGuid)
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&settings).Error
}

func (s *UserSettingsService) Get(userGuid string) (*domains.UserSettings, error) {
	userGuid = strings.TrimSpace(userGuid)
	if userGuid == "" {
		return nil, errors.New("user guid is required")
	}
	var settings domains.UserSettings
	err := s.DB().Where("user_guid = ?", userGuid).First(&settings).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := s.DB().Transaction(func(tx *gorm.DB) error {
			return s.Ensure(tx, userGuid)
		}); err != nil {
			return nil, err
		}
		err = s.DB().Where("user_guid = ?", userGuid).First(&settings).Error
	}
	if err != nil {
		return nil, err
	}
	normalizeUserSettings(&settings)
	return &settings, nil
}

func (s *UserSettingsService) Save(userGuid string, settings *domains.UserSettings) (*domains.UserSettings, error) {
	userGuid = strings.TrimSpace(userGuid)
	if userGuid == "" {
		return nil, errors.New("user guid is required")
	}
	if settings == nil {
		return nil, errors.New("user settings is required")
	}
	updating := *settings
	updating.UserGuid = userGuid
	normalizeUserSettings(&updating)
	if err := validateOptionalJSONObject(updating.ExtraConfig, "extraConfig"); err != nil {
		return nil, err
	}
	if err := s.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.Ensure(tx, userGuid); err != nil {
			return err
		}
		return tx.Model(&domains.UserSettings{}).Where("user_guid = ?", userGuid).Updates(map[string]any{
			"quota_reminder_enabled":        updating.QuotaReminderEnabled,
			"platform_announcement_enabled": updating.PlatformAnnouncementEnabled,
			"abnormal_call_alert_enabled":   updating.AbnormalCallAlertEnabled,
			"max_concurrency":               updating.MaxConcurrency,
			"extra_config":                  updating.ExtraConfig,
		}).Error
	}); err != nil {
		return nil, err
	}
	return s.Get(userGuid)
}

func (s *UserSettingsService) SavePreferences(userGuid string, settings *domains.UserSettings) (*domains.UserSettings, error) {
	userGuid = strings.TrimSpace(userGuid)
	if userGuid == "" {
		return nil, errors.New("user guid is required")
	}
	if settings == nil {
		return nil, errors.New("user settings is required")
	}
	updating := *settings
	updating.UserGuid = userGuid
	normalizeUserSettings(&updating)
	if err := validateOptionalJSONObject(updating.ExtraConfig, "extraConfig"); err != nil {
		return nil, err
	}
	if err := s.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.Ensure(tx, userGuid); err != nil {
			return err
		}
		return tx.Model(&domains.UserSettings{}).Where("user_guid = ?", userGuid).Updates(map[string]any{
			"quota_reminder_enabled":        updating.QuotaReminderEnabled,
			"platform_announcement_enabled": updating.PlatformAnnouncementEnabled,
			"abnormal_call_alert_enabled":   updating.AbnormalCallAlertEnabled,
			"extra_config":                  updating.ExtraConfig,
		}).Error
	}); err != nil {
		return nil, err
	}
	return s.Get(userGuid)
}

func defaultUserSettings(userGuid string) domains.UserSettings {
	return domains.UserSettings{
		UserGuid:                    userGuid,
		QuotaReminderEnabled:        true,
		PlatformAnnouncementEnabled: true,
		AbnormalCallAlertEnabled:    false,
		MaxConcurrency:              DefaultUserMaxConcurrency,
		ExtraConfig:                 "{}",
	}
}

func normalizeUserSettings(settings *domains.UserSettings) {
	settings.UserGuid = strings.TrimSpace(settings.UserGuid)
	settings.ExtraConfig = strings.TrimSpace(settings.ExtraConfig)
	if settings.ExtraConfig == "" {
		settings.ExtraConfig = "{}"
	}
	if settings.MaxConcurrency <= 0 {
		settings.MaxConcurrency = DefaultUserMaxConcurrency
	}
}
