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
