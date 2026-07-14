package services

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"navapi-go/constants"
	"navapi-go/domains"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestModelGroupProviderScopeAllKeepsExistingRouting(t *testing.T) {
	modelService, providerService, db := newModelGroupProviderTestServices(t)
	first := createRoutingTestProvider(t, db, "first", "gpt-test", true)
	second := createRoutingTestProvider(t, db, "second", "gpt-test", true)

	candidates, err := providerService.FindCandidatesForModelAndType("gpt-test", constants.DefaultGroup, constants.ProviderTypeOpenAI)
	if err != nil {
		t.Fatalf("find candidates: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected both providers, got %d", len(candidates))
	}

	groups, err := modelService.ListGroups(true)
	if err != nil {
		t.Fatalf("list groups: %v", err)
	}
	if len(groups) != 1 || groups[0].ProviderScope != constants.ModelGroupProviderScopeAll {
		t.Fatalf("expected default group with all scope, got %+v", groups)
	}
	assertProviderGuids(t, candidates, first.Guid, second.Guid)
}

func TestModelGroupProviderScopeSelectedRestrictsCandidatesAndCanBeUpdated(t *testing.T) {
	modelService, providerService, db := newModelGroupProviderTestServices(t)
	first := createRoutingTestProvider(t, db, "first", "gpt-test", true)
	second := createRoutingTestProvider(t, db, "second", "gpt-test", true)
	group := domains.ModelGroup{
		GroupName:       "pro",
		DisplayName:     "Pro",
		QuotaMultiplier: 1,
		ProviderScope:   constants.ModelGroupProviderScopeSelected,
		ProviderGuids:   []string{first.Guid},
		Enabled:         true,
	}
	if err := modelService.UpsertGroup(&group); err != nil {
		t.Fatalf("create selected group: %v", err)
	}
	if group.ProviderCount != 1 || len(group.ProviderGuids) != 1 || group.ProviderGuids[0] != first.Guid {
		t.Fatalf("unexpected saved provider selection: %+v", group)
	}

	candidates, err := providerService.FindCandidatesForModelAndType("gpt-test", group.GroupName, constants.ProviderTypeOpenAI)
	if err != nil {
		t.Fatalf("find selected candidates: %v", err)
	}
	assertProviderGuids(t, candidates, first.Guid)

	group.ProviderGuids = []string{second.Guid}
	if err := modelService.UpsertGroup(&group); err != nil {
		t.Fatalf("update selected group: %v", err)
	}
	candidates, err = providerService.FindCandidatesForModelAndType("gpt-test", group.GroupName, constants.ProviderTypeOpenAI)
	if err != nil {
		t.Fatalf("find updated candidates: %v", err)
	}
	assertProviderGuids(t, candidates, second.Guid)
}

func TestModelGroupProviderScopeSelectedNeverFallsBackOutsideGroup(t *testing.T) {
	modelService, providerService, db := newModelGroupProviderTestServices(t)
	selected := createRoutingTestProvider(t, db, "selected", "model-a", true)
	createRoutingTestProvider(t, db, "outside", "model-b", true)
	group := domains.ModelGroup{
		GroupName:       "strict",
		QuotaMultiplier: 1,
		ProviderScope:   constants.ModelGroupProviderScopeSelected,
		ProviderGuids:   []string{selected.Guid},
		Enabled:         true,
	}
	if err := modelService.UpsertGroup(&group); err != nil {
		t.Fatalf("create selected group: %v", err)
	}

	_, err := providerService.FindCandidatesForModelAndType("model-b", group.GroupName, constants.ProviderTypeOpenAI)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected no candidate instead of outside fallback, got %v", err)
	}

	if err := db.Model(&domains.VendorMeta{}).Where("guid = ?", selected.Guid).Update("enabled", false).Error; err != nil {
		t.Fatalf("disable selected provider: %v", err)
	}
	_, err = providerService.FindCandidatesForModelAndType("model-a", group.GroupName, constants.ProviderTypeOpenAI)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected disabled selected provider to be unavailable, got %v", err)
	}
}

func TestModelGroupProviderScopeFiltersModelDiscovery(t *testing.T) {
	modelService, providerService, db := newModelGroupProviderTestServices(t)
	selected := createRoutingTestProvider(t, db, "selected", "model-a,model-shared", true)
	createRoutingTestProvider(t, db, "outside", "model-b,model-shared", true)
	group := domains.ModelGroup{
		GroupName:       "models",
		QuotaMultiplier: 1,
		ProviderScope:   constants.ModelGroupProviderScopeSelected,
		ProviderGuids:   []string{selected.Guid},
		Enabled:         true,
	}
	if err := modelService.UpsertGroup(&group); err != nil {
		t.Fatalf("create selected group: %v", err)
	}

	models, err := providerService.ListEnabledModelsForGroupAndType(group.GroupName, constants.ProviderTypeOpenAI)
	if err != nil {
		t.Fatalf("list group models: %v", err)
	}
	if strings.Join(models, ",") != "model-a,model-shared" {
		t.Fatalf("unexpected provider model list: %v", models)
	}
	response, err := modelService.ListOpenAIModelsForGroup(group.GroupName)
	if err != nil {
		t.Fatalf("list OpenAI models: %v", err)
	}
	if len(response.Data) != 2 || response.Data[0].ID != "model-a" || response.Data[1].ID != "model-shared" {
		t.Fatalf("unexpected OpenAI model response: %+v", response.Data)
	}
}

func TestModelGroupProviderScopeValidatesSelection(t *testing.T) {
	modelService, _, _ := newModelGroupProviderTestServices(t)
	empty := domains.ModelGroup{
		GroupName:       "empty",
		ProviderScope:   constants.ModelGroupProviderScopeSelected,
		QuotaMultiplier: 1,
		Enabled:         true,
	}
	if err := modelService.UpsertGroup(&empty); err == nil {
		t.Fatal("expected empty selected provider scope to fail")
	}

	missing := domains.ModelGroup{
		GroupName:       "missing",
		ProviderScope:   constants.ModelGroupProviderScopeSelected,
		ProviderGuids:   []string{"missing-provider-guid"},
		QuotaMultiplier: 1,
		Enabled:         true,
	}
	if err := modelService.UpsertGroup(&missing); err == nil {
		t.Fatal("expected missing provider guid to fail")
	}
}

func newModelGroupProviderTestServices(t *testing.T) (*ModelService, *ProviderService, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.AutoMigrate(&domains.ModelGroup{}, &domains.VendorMeta{}, &domains.ModelGroupProvider{}, &domains.ModelMeta{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	return ModelServiceApp.WithDB(db), ProviderServiceApp.WithDB(db), db
}

func createRoutingTestProvider(t *testing.T, db *gorm.DB, name string, models string, enabled bool) domains.VendorMeta {
	t.Helper()
	provider := domains.VendorMeta{
		VendorName:  name,
		DisplayName: name,
		Type:        constants.ProviderTypeOpenAI,
		Key:         "test-key",
		Models:      models,
		Enabled:     enabled,
	}
	if err := db.Create(&provider).Error; err != nil {
		t.Fatalf("create provider %s: %v", name, err)
	}
	return provider
}

func assertProviderGuids(t *testing.T, providers []domains.VendorMeta, expected ...string) {
	t.Helper()
	actual := make(map[string]struct{}, len(providers))
	for _, provider := range providers {
		actual[provider.Guid] = struct{}{}
	}
	if len(actual) != len(expected) {
		t.Fatalf("expected %d provider guids, got %v", len(expected), actual)
	}
	for _, guid := range expected {
		if _, ok := actual[guid]; !ok {
			t.Fatalf("expected provider %s, got %v", guid, actual)
		}
	}
}
