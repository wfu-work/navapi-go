package services

import (
	"strings"
	"testing"
	"time"

	"navapi-go/constants"
	"navapi-go/domains"

	"github.com/wfu-work/nav-common-go-lib/global"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAnnouncementSaveValidatesAndNormalizes(t *testing.T) {
	withAnnouncementTestDB(t)

	announcement := domains.Announcement{
		Title:   "  Maintenance window  ",
		Content: "  Scheduled work tonight.  ",
	}

	if err := AnnouncementServiceApp.Save(&announcement); err != nil {
		t.Fatal(err)
	}
	if announcement.Id == 0 {
		t.Fatal("announcement id was not set")
	}
	if announcement.Title != "Maintenance window" {
		t.Fatalf("title = %q", announcement.Title)
	}
	if announcement.Content != "Scheduled work tonight." {
		t.Fatalf("content = %q", announcement.Content)
	}
	if announcement.Level != "info" {
		t.Fatalf("level = %q, want info", announcement.Level)
	}
	if announcement.Status != constants.StatusEnabled {
		t.Fatalf("status = %d, want enabled", announcement.Status)
	}

	err := AnnouncementServiceApp.Save(&domains.Announcement{Title: "missing content"})
	if err == nil || !strings.Contains(err.Error(), "content is required") {
		t.Fatalf("err = %v, want content validation", err)
	}
}

func TestAnnouncementSaveRequiresExistingRowOnUpdate(t *testing.T) {
	withAnnouncementTestDB(t)

	missing := domains.Announcement{
		Title:   "Missing",
		Content: "Missing",
		Level:   "info",
		Status:  constants.StatusEnabled,
	}
	missing.Id = 999
	err := AnnouncementServiceApp.Save(&missing)
	if err == nil || !strings.Contains(err.Error(), "announcement not found") {
		t.Fatalf("err = %v, want not found", err)
	}
}

func TestAnnouncementPublicListOnlyReturnsActiveAnnouncements(t *testing.T) {
	withAnnouncementTestDB(t)
	now := time.Now().Unix()
	records := []domains.Announcement{
		{Title: "Active", Content: "visible", Level: "info", Status: constants.StatusEnabled, StartTime: now - 10, EndTime: now + 10},
		{Title: "Disabled", Content: "hidden", Level: "info", Status: constants.StatusDisabled},
		{Title: "Future", Content: "hidden", Level: "info", Status: constants.StatusEnabled, StartTime: now + 100},
		{Title: "Expired", Content: "hidden", Level: "info", Status: constants.StatusEnabled, EndTime: now - 100},
	}
	if err := global.NAV_DB.Create(&records).Error; err != nil {
		t.Fatal(err)
	}

	result, err := AnnouncementServiceApp.List(AnnouncementQuery{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Fatalf("total = %d, want 1", result.Total)
	}
	list := result.List.([]domains.Announcement)
	if len(list) != 1 || list[0].Title != "Active" {
		t.Fatalf("list = %+v, want only Active", list)
	}
}

func TestAnnouncementListSupportsAdminFilters(t *testing.T) {
	withAnnouncementTestDB(t)
	popup := true
	records := []domains.Announcement{
		{Title: "Popup warning", Content: "one", Level: "warning", Status: constants.StatusEnabled, Popup: true},
		{Title: "Inline warning", Content: "two", Level: "warning", Status: constants.StatusEnabled, Popup: false},
		{Title: "Popup info", Content: "three", Level: "info", Status: constants.StatusDisabled, Popup: true},
	}
	if err := global.NAV_DB.Create(&records).Error; err != nil {
		t.Fatal(err)
	}

	result, err := AnnouncementServiceApp.List(AnnouncementQuery{
		Level:  "warning",
		Status: constants.StatusEnabled,
		Popup:  &popup,
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Fatalf("total = %d, want 1", result.Total)
	}
	list := result.List.([]domains.Announcement)
	if len(list) != 1 || list[0].Title != "Popup warning" {
		t.Fatalf("list = %+v, want only popup warning", list)
	}
}

func withAnnouncementTestDB(t *testing.T) {
	t.Helper()
	previous := global.NAV_DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domains.Announcement{}); err != nil {
		t.Fatal(err)
	}
	global.NAV_DB = db
	t.Cleanup(func() {
		global.NAV_DB = previous
	})
}
