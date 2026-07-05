package services

import (
	"testing"

	"navapi-go/domains"

	"github.com/wfu-work/nav-common-go-lib/global"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUserSettingsGetCreatesDefaults(t *testing.T) {
	withUserSettingsTestDB(t)

	settings, err := UserSettingsServiceApp.Get("user-001")
	if err != nil {
		t.Fatal(err)
	}
	if settings.UserGuid != "user-001" ||
		!settings.QuotaReminderEnabled ||
		!settings.PlatformAnnouncementEnabled ||
		settings.AbnormalCallAlertEnabled ||
		settings.MaxConcurrency != DefaultUserMaxConcurrency ||
		settings.ExtraConfig != "{}" {
		t.Fatalf("settings = %+v, want defaults", settings)
	}
}

func TestUserSettingsSavePersistsBooleanValues(t *testing.T) {
	withUserSettingsTestDB(t)

	settings, err := UserSettingsServiceApp.Save("user-002", &domains.UserSettings{
		QuotaReminderEnabled:        false,
		PlatformAnnouncementEnabled: false,
		AbnormalCallAlertEnabled:    true,
		MaxConcurrency:              8,
		ExtraConfig:                 `{"theme":"compact"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if settings.QuotaReminderEnabled ||
		settings.PlatformAnnouncementEnabled ||
		!settings.AbnormalCallAlertEnabled ||
		settings.MaxConcurrency != 8 ||
		settings.ExtraConfig != `{"theme":"compact"}` {
		t.Fatalf("settings = %+v, want saved values", settings)
	}
}

func withUserSettingsTestDB(t *testing.T) {
	t.Helper()
	previousDB := global.NAV_DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domains.UserSettings{}); err != nil {
		t.Fatal(err)
	}
	global.NAV_DB = db
	UserConcurrencyServiceApp.reset()
	t.Cleanup(func() {
		global.NAV_DB = previousDB
		UserConcurrencyServiceApp.reset()
	})
}
