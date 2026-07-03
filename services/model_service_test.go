package services

import (
	"strings"
	"testing"

	"navapi-go/constants"
	"navapi-go/domains"

	"github.com/wfu-work/nav-common-go-lib/global"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestModelUpsertMetaUpdatesExistingRow(t *testing.T) {
	withModelTestDB(t)

	model := domains.ModelMeta{
		ModelName:   "gpt-4o-mini",
		DisplayName: "GPT-4o Mini",
		OwnedBy:     "openai",
		Enabled:     true,
		Sort:        10,
		ContextSize: 128000,
		Remark:      "old",
	}
	if err := ModelServiceApp.UpsertMeta(&model); err != nil {
		t.Fatal(err)
	}
	if model.Id == 0 {
		t.Fatal("model id was not set")
	}
	if model.Guid == "" {
		t.Fatal("model guid was not set")
	}
	originalID := model.Id
	originalGuid := model.Guid

	update := domains.ModelMeta{
		BaseDataEntity: model.BaseDataEntity,
		ModelName:      "gpt-4o-mini",
		DisplayName:    "GPT-4o Mini Updated",
		OwnedBy:        "navapi",
		Enabled:        false,
		Sort:           0,
		ContextSize:    0,
		Remark:         "",
	}
	update.Id = 0
	if err := ModelServiceApp.UpsertMeta(&update); err != nil {
		t.Fatal(err)
	}
	if update.Id != originalID {
		t.Fatalf("updated id = %d, want %d", update.Id, originalID)
	}
	if update.Guid != originalGuid {
		t.Fatalf("updated guid = %q, want %q", update.Guid, originalGuid)
	}

	var count int64
	if err := global.NAV_DB.Model(&domains.ModelMeta{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("model count = %d, want 1", count)
	}

	reloaded, err := ModelServiceApp.GetById(originalID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded == nil {
		t.Fatal("updated model was not found")
	}
	if reloaded.DisplayName != "GPT-4o Mini Updated" {
		t.Fatalf("displayName = %q, want updated value", reloaded.DisplayName)
	}
	if reloaded.OwnedBy != "navapi" {
		t.Fatalf("ownedBy = %q, want navapi", reloaded.OwnedBy)
	}
	if reloaded.Enabled {
		t.Fatal("enabled = true, want false")
	}
	if reloaded.Sort != 0 || reloaded.ContextSize != 0 {
		t.Fatalf("zero values were not saved: sort=%d contextSize=%d", reloaded.Sort, reloaded.ContextSize)
	}
}

func TestModelDeleteMetaUsesGuid(t *testing.T) {
	withModelTestDB(t)

	model := domains.ModelMeta{
		ModelName: "gpt-4o-mini",
		Enabled:   true,
	}
	if err := ModelServiceApp.UpsertMeta(&model); err != nil {
		t.Fatal(err)
	}
	if model.Guid == "" {
		t.Fatal("model guid was not set")
	}

	if err := ModelServiceApp.DeleteMeta(model.Guid); err != nil {
		t.Fatal(err)
	}

	var count int64
	if err := global.NAV_DB.Model(&domains.ModelMeta{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("model count = %d, want 0", count)
	}
}

func TestTokenCheckModelRequiresMatchingModelGroup(t *testing.T) {
	withModelTestDB(t)

	pro := domains.ModelGroup{
		GroupName:       "pro",
		DisplayName:     "专业版",
		QuotaMultiplier: 2,
		Enabled:         true,
	}
	if err := ModelServiceApp.UpsertGroup(&pro); err != nil {
		t.Fatal(err)
	}
	model := domains.ModelMeta{
		ModelName: "gpt-4o",
		Group:     "pro",
		Enabled:   true,
	}
	if err := ModelServiceApp.UpsertMeta(&model); err != nil {
		t.Fatal(err)
	}

	if err := TokenServiceApp.CheckModel(&domains.ApiToken{Group: "pro"}, "gpt-4o"); err != nil {
		t.Fatal(err)
	}
	err := TokenServiceApp.CheckModel(&domains.ApiToken{Group: constants.DefaultGroup}, "gpt-4o")
	if err == nil || !strings.Contains(err.Error(), "model is not allowed") {
		t.Fatalf("err = %v, want model group validation", err)
	}
}

func withModelTestDB(t *testing.T) {
	t.Helper()
	previous := global.NAV_DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domains.ModelMeta{}, &domains.ModelGroup{}); err != nil {
		t.Fatal(err)
	}
	global.NAV_DB = db
	if err := ModelServiceApp.EnsureDefaultGroup(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		global.NAV_DB = previous
	})
}
