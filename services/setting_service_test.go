package services

import (
	"testing"

	"navapi-go/domains"

	"github.com/wfu-work/nav-common-go-lib/global"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSettingContactSettingsSaveAndClear(t *testing.T) {
	withSettingTestDB(t)

	saved, err := SettingServiceApp.SaveContactSettings(ContactSettings{
		QQGroupNo:     "123456",
		QQGroupQRCode: "https://example.com/qq.png",
		WechatAccount: "navapi",
		WechatQRCode:  "https://example.com/wechat.png",
		SponsorQRCode: "https://example.com/sponsor.png",
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.QQGroupNo != "123456" || saved.WechatAccount != "navapi" || saved.SponsorQRCode == "" {
		t.Fatalf("saved settings = %+v, want contact values", saved)
	}

	cleared, err := SettingServiceApp.SaveContactSettings(ContactSettings{
		QQGroupNo:     "",
		QQGroupQRCode: "",
		WechatAccount: "navapi-new",
		WechatQRCode:  "",
		SponsorQRCode: "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cleared.QQGroupNo != "" || cleared.QQGroupQRCode != "" || cleared.WechatAccount != "navapi-new" || cleared.SponsorQRCode != "" {
		t.Fatalf("cleared settings = %+v, want empty values persisted", cleared)
	}
}

func TestRegisterSettingsDefaultAndSaveUseSetting(t *testing.T) {
	db := withSettingTestDB(t)

	defaults := RegisterSettingServiceApp.Get()
	if !defaults.Enabled || !defaults.RequireCaptcha || defaults.RequireInvite ||
		defaults.DefaultAmount != defaultRegisterAmount ||
		defaults.DefaultGroup != "default" ||
		defaults.Notice != defaultRegisterNotice {
		t.Fatalf("defaults = %+v, want open registration, captcha and quota defaults", defaults)
	}

	err := RegisterSettingServiceApp.Set(RegisterSettings{
		Enabled:        false,
		DefaultAmount:  25,
		DefaultGroup:   " vip ",
		AllowedGroups:  " default, vip ,, enterprise ",
		RequireInvite:  true,
		RequireCaptcha: false,
		Notice:         " 内测开放 ",
	})
	if err != nil {
		t.Fatal(err)
	}
	saved := RegisterSettingServiceApp.Get()
	if saved.Enabled ||
		saved.DefaultAmount != 25 ||
		saved.DefaultGroup != "vip" ||
		saved.AllowedGroups != "default,vip,enterprise" ||
		!saved.RequireInvite ||
		saved.RequireCaptcha ||
		saved.Notice != "内测开放" {
		t.Fatalf("saved = %+v, want normalized setting values", saved)
	}
	var row domains.Setting
	if err := db.Where("key = ?", settingRegisterDefaultAmount).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Value != "25" {
		t.Fatalf("default quota setting = %+v, want value 25", row)
	}
}

func withSettingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := global.NAV_DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domains.Setting{}); err != nil {
		t.Fatal(err)
	}
	global.NAV_DB = db
	t.Cleanup(func() {
		global.NAV_DB = previousDB
	})
	return db
}
