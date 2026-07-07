package services

import (
	"testing"
	"time"

	"navapi-go/domains"

	commonDomains "github.com/wfu-work/nav-common-go-lib/domains"
)

func TestGatewayPublicStatusBuildsModelProbeData(t *testing.T) {
	db := withLogTestDB(t)
	now := time.Now().UnixMilli()
	models := []domains.ModelMeta{
		{BaseDataEntity: commonDomains.BaseDataEntity{Guid: "model-active"}, ModelName: "gpt-5.5", DisplayName: "GPT-5.5", Enabled: true},
		{BaseDataEntity: commonDomains.BaseDataEntity{Guid: "model-idle"}, ModelName: "gpt-idle", DisplayName: "GPT Idle", Enabled: true},
		{BaseDataEntity: commonDomains.BaseDataEntity{Guid: "model-disabled"}, ModelName: "gpt-disabled", Enabled: false},
	}
	if err := db.Create(&models).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&domains.ModelMeta{}).Where("model_name = ?", "gpt-disabled").Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	logs := []domains.UsageLog{
		{BaseDataEntity: commonDomains.BaseDataEntity{CreateTime: now - 30*60*1000}, ModelName: "gpt-5.5", Status: "success", UseTimeMs: 600},
		{BaseDataEntity: commonDomains.BaseDataEntity{CreateTime: now - 20*60*1000}, ModelName: "gpt-5.5", Status: "success", UseTimeMs: 800},
		{BaseDataEntity: commonDomains.BaseDataEntity{CreateTime: now - 10*60*1000}, ModelName: "gpt-5.5", Status: "error", UseTimeMs: 1200},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatal(err)
	}

	status, err := GatewayServiceApp.PublicStatus("test")
	if err != nil {
		t.Fatal(err)
	}
	if status.Summary.EnabledModels != 2 {
		t.Fatalf("enabled models = %d, want 2", status.Summary.EnabledModels)
	}
	if status.Summary.TotalRequests != 3 || status.Summary.SuccessRequests != 2 || status.Summary.ErrorRequests != 1 {
		t.Fatalf("summary = %+v, want aggregated recent logs", status.Summary)
	}

	active := findPublicModelStatus(status.Models, "gpt-5.5")
	if active == nil {
		t.Fatalf("models = %+v, want gpt-5.5", status.Models)
	}
	if active.DisplayName != "GPT-5.5" || active.Requests != 3 || active.LastCheckedAt == 0 {
		t.Fatalf("active model = %+v, want display name and requests", active)
	}
	if len(active.Segments) != serviceStatusSegmentCount {
		t.Fatalf("segments = %d, want %d", len(active.Segments), serviceStatusSegmentCount)
	}

	idle := findPublicModelStatus(status.Models, "gpt-idle")
	if idle == nil || idle.Status != "idle" || idle.Requests != 0 {
		t.Fatalf("idle model = %+v, want idle without requests", idle)
	}
	if findPublicModelStatus(status.Models, "gpt-disabled") != nil {
		t.Fatalf("models = %+v, want disabled model hidden", status.Models)
	}
}

func findPublicModelStatus(items []PublicModelStatus, modelName string) *PublicModelStatus {
	for i := range items {
		if items[i].ModelName == modelName {
			return &items[i]
		}
	}
	return nil
}
