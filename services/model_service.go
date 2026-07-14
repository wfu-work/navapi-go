package services

import (
	"errors"
	"sort"
	"strings"
	"time"

	"navapi-go/constants"
	"navapi-go/domains"
	"navapi-go/vos"

	commonServices "github.com/wfu-work/nav-common-go-lib/services"
	"gorm.io/gorm"
)

type ModelService struct {
	commonServices.CrudService[domains.ModelMeta]
	VendorCrud commonServices.CrudService[domains.VendorMeta]
	GroupCrud  commonServices.CrudService[domains.ModelGroup]
}

var ModelServiceApp = new(ModelService)

func (s *ModelService) WithDB(db *gorm.DB) *ModelService {
	cloned := *s
	cloned.CrudService = *s.CrudService.WithDB(db)
	cloned.VendorCrud = *s.VendorCrud.WithDB(db)
	cloned.GroupCrud = *s.GroupCrud.WithDB(db)
	return &cloned
}

func (s *ModelService) ListOpenAIModels() (vos.ModelListResponse, error) {
	return s.listOpenAIModels("*", "")
}

func (s *ModelService) ListOpenAIModelsForGroup(group string) (vos.ModelListResponse, error) {
	return s.listOpenAIModels(group, constants.ProviderTypeOpenAI)
}

func (s *ModelService) listOpenAIModels(group string, providerType string) (vos.ModelListResponse, error) {
	models, err := ProviderServiceApp.WithDB(s.DB()).ListEnabledModelsForGroupAndType(group, providerType)
	if err != nil {
		return vos.ModelListResponse{}, err
	}
	metas, err := s.ListMeta()
	if err != nil {
		return vos.ModelListResponse{}, err
	}
	metaByModel := map[string]domains.ModelMeta{}
	for _, meta := range metas {
		metaByModel[meta.ModelName] = meta
	}
	sort.Strings(models)
	data := make([]vos.ModelInfo, 0, len(models))
	now := time.Now().Unix()
	for _, model := range models {
		ownedBy := "navapi-go"
		if meta, ok := metaByModel[model]; ok {
			if !meta.Enabled {
				continue
			}
			if meta.OwnedBy != "" {
				ownedBy = meta.OwnedBy
			}
		}
		data = append(data, vos.ModelInfo{
			ID:      model,
			Object:  "model",
			Created: now,
			OwnedBy: ownedBy,
		})
	}
	return vos.ModelListResponse{Object: "list", Data: data}, nil
}

func (s *ModelService) UpsertMeta(meta *domains.ModelMeta) error {
	normalizeModelMeta(meta)
	if err := s.ValidateModelGroups(meta.Groups); err != nil {
		return err
	}
	if meta.Guid == "" {
		meta.Id = 0
		if err := createWithCrud(&s.CrudService, meta); err != nil {
			return err
		}
		fillModelMetaGroups(meta)
		return nil
	}
	existing, err := s.GetByGuid(meta.Guid)
	if err != nil {
		return err
	}
	if existing == nil {
		return errors.New("model not found")
	}
	meta.Id = existing.Id
	meta.Guid = existing.Guid
	meta.CreateTime = existing.CreateTime
	meta.Creater = existing.Creater
	meta.Updater = existing.Updater
	meta.UpdateTime = time.Now().UnixMilli()
	if err := s.DB().Save(meta).Error; err != nil {
		return err
	}
	if err := reloadByGuidWithCrud(&s.CrudService, meta); err != nil {
		return err
	}
	fillModelMetaGroups(meta)
	return nil
}

func (s *ModelService) ListMeta() ([]domains.ModelMeta, error) {
	var metas []domains.ModelMeta
	if err := s.DB().Order("sort desc, id desc").Find(&metas).Error; err != nil {
		return nil, err
	}
	for i := range metas {
		fillModelMetaGroups(&metas[i])
	}
	return metas, nil
}

func (s *ModelService) PublicListMeta() ([]domains.ModelMeta, error) {
	var metas []domains.ModelMeta
	if err := s.DB().Where("enabled = ?", true).Order("sort desc, id desc").Find(&metas).Error; err != nil {
		return nil, err
	}
	for i := range metas {
		fillModelMetaGroups(&metas[i])
	}
	return metas, nil
}

func (s *ModelService) DeleteMeta(guid string) error {
	guid = strings.TrimSpace(guid)
	if guid == "" {
		return errors.New("guid is required")
	}
	existing, err := s.GetByGuid(guid)
	if err != nil {
		return err
	}
	if existing == nil {
		return errors.New("model not found")
	}
	return s.DeleteByGuid(guid)
}

func (s *ModelService) ListGroups(includeDisabled bool) ([]domains.ModelGroup, error) {
	if err := s.EnsureDefaultGroup(); err != nil {
		return nil, err
	}
	var groups []domains.ModelGroup
	db := s.GroupCrud.DB().Order("sort desc, group_name asc, id desc")
	if !includeDisabled {
		db = db.Where("enabled = ?", true)
	}
	if err := db.Find(&groups).Error; err != nil {
		return nil, err
	}
	if err := s.fillModelGroupProviders(groups); err != nil {
		return nil, err
	}
	return groups, nil
}

func (s *ModelService) UpsertGroup(group *domains.ModelGroup) error {
	normalizeModelGroup(group)
	providerScope := group.ProviderScope
	providerGuids := append([]string(nil), group.ProviderGuids...)
	if providerScope == constants.ModelGroupProviderScopeSelected && len(providerGuids) == 0 {
		return errors.New("at least one provider is required for selected provider scope")
	}
	db := s.GroupCrud.DB()
	if db == nil {
		return errors.New("database is not initialized")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		service := s.WithDB(tx)
		if err := service.validateGroupProviders(providerGuids); err != nil {
			return err
		}
		if group.Guid == "" {
			group.Id = 0
			if err := createWithCrud(&service.GroupCrud, group); err != nil {
				return err
			}
		} else {
			existing, err := service.GroupCrud.GetByGuid(group.Guid)
			if err != nil {
				return err
			}
			if existing == nil {
				return errors.New("model group not found")
			}
			group.GroupName = existing.GroupName
			group.Id = existing.Id
			group.Guid = existing.Guid
			group.CreateTime = existing.CreateTime
			group.Creater = existing.Creater
			group.Updater = existing.Updater
			group.UpdateTime = time.Now().UnixMilli()
			if err := service.GroupCrud.DB().Save(group).Error; err != nil {
				return err
			}
			if err := reloadByGuidWithCrud(&service.GroupCrud, group); err != nil {
				return err
			}
		}
		if err := service.replaceGroupProviders(group.Guid, providerScope, providerGuids); err != nil {
			return err
		}
		return service.fillModelGroupProvider(group)
	})
}

func (s *ModelService) DeleteGroup(guid string) error {
	guid = strings.TrimSpace(guid)
	if guid == "" {
		return errors.New("guid is required")
	}
	group, err := s.GroupCrud.GetByGuid(guid)
	if err != nil {
		return err
	}
	if group == nil {
		return errors.New("model group not found")
	}
	if normalizeGroup(group.GroupName) == "default" {
		return errors.New("default group cannot be deleted")
	}
	var count int64
	var metas []domains.ModelMeta
	if err := s.DB().Model(&domains.ModelMeta{}).Find(&metas).Error; err != nil {
		return err
	}
	groupName := normalizeGroup(group.GroupName)
	for _, meta := range metas {
		if modelGroupsContain(splitModelGroups(meta.Group), groupName) {
			count++
			break
		}
	}
	if count > 0 {
		return errors.New("model group is in use")
	}
	db := s.GroupCrud.DB()
	if db == nil {
		return errors.New("database is not initialized")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("group_guid = ?", group.Guid).Delete(&domains.ModelGroupProvider{}).Error; err != nil {
			return err
		}
		return s.WithDB(tx).GroupCrud.DeleteByGuid(guid)
	})
}

func (s *ModelService) EnsureDefaultGroup() error {
	db := s.GroupCrud.DB()
	if db == nil {
		return errors.New("database is not initialized")
	}
	var count int64
	if err := db.Model(&domains.ModelGroup{}).Where("group_name = ?", constants.DefaultGroup).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	group := domains.ModelGroup{
		GroupName:       constants.DefaultGroup,
		DisplayName:     "默认分组",
		QuotaMultiplier: 1,
		ProviderScope:   constants.ModelGroupProviderScopeAll,
		Enabled:         true,
	}
	return createWithCrud(&s.GroupCrud, &group)
}

func (s *ModelService) ValidateGroup(group string) error {
	group = normalizeGroup(group)
	if group == "*" {
		return nil
	}
	if err := s.EnsureDefaultGroup(); err != nil {
		return err
	}
	modelGroup, err := s.groupByName(group)
	if err != nil {
		return err
	}
	if modelGroup == nil {
		return errors.New("model group not found")
	}
	if !modelGroup.Enabled {
		return errors.New("model group is disabled")
	}
	return nil
}

func (s *ModelService) ValidateModelGroups(groups []string) error {
	groups = normalizeModelGroups(groups)
	for _, group := range groups {
		if err := s.ValidateGroup(group); err != nil {
			return err
		}
	}
	return nil
}

func (s *ModelService) ModelAllowedForGroup(modelName string, group string) error {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return errors.New("model is required")
	}
	group = normalizeGroup(group)
	if err := s.ValidateGroup(group); err != nil {
		return err
	}
	var metas []domains.ModelMeta
	if err := s.DB().Where("model_name = ?", modelName).Find(&metas).Error; err != nil {
		return err
	}
	if len(metas) == 0 {
		return nil
	}
	for _, meta := range metas {
		if !meta.Enabled {
			continue
		}
		if modelGroupsContain(splitModelGroups(meta.Group), group) {
			return nil
		}
	}
	return errors.New("model is not allowed in token group")
}

func (s *ModelService) GroupQuotaMultiplier(group string) float64 {
	modelGroup, err := s.groupByName(normalizeGroup(group))
	if err != nil || modelGroup == nil || !modelGroup.Enabled || modelGroup.QuotaMultiplier <= 0 {
		return 1
	}
	return modelGroup.QuotaMultiplier
}

func (s *ModelService) AllowedProviderGuidsForGroup(group string) (string, map[string]struct{}, error) {
	group = normalizeGroup(group)
	if group == "*" {
		return constants.ModelGroupProviderScopeAll, nil, nil
	}
	if err := s.EnsureDefaultGroup(); err != nil {
		return "", nil, err
	}
	modelGroup, err := s.groupByName(group)
	if err != nil {
		return "", nil, err
	}
	if modelGroup == nil {
		return "", nil, errors.New("model group not found")
	}
	if !modelGroup.Enabled {
		return "", nil, errors.New("model group is disabled")
	}
	scope := normalizeProviderScope(modelGroup.ProviderScope)
	if scope == constants.ModelGroupProviderScopeAll {
		return scope, nil, nil
	}
	var providerGuids []string
	if err := s.GroupCrud.DB().Model(&domains.ModelGroupProvider{}).
		Where("group_guid = ?", modelGroup.Guid).
		Order("sort asc, id asc").
		Pluck("provider_guid", &providerGuids).Error; err != nil {
		return "", nil, err
	}
	allowed := make(map[string]struct{}, len(providerGuids))
	for _, providerGuid := range providerGuids {
		allowed[providerGuid] = struct{}{}
	}
	return scope, allowed, nil
}

func (s *ModelService) groupByName(group string) (*domains.ModelGroup, error) {
	group = normalizeGroup(group)
	var result domains.ModelGroup
	err := s.GroupCrud.DB().Where("group_name = ?", group).First(&result).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &result, nil
}

func normalizeModelGroup(group *domains.ModelGroup) {
	group.Guid = strings.TrimSpace(group.Guid)
	group.GroupName = normalizeGroup(group.GroupName)
	group.DisplayName = strings.TrimSpace(group.DisplayName)
	group.Remark = strings.TrimSpace(group.Remark)
	group.ProviderScope = normalizeProviderScope(group.ProviderScope)
	group.ProviderGuids = normalizeProviderGuids(group.ProviderGuids)
	if group.ProviderScope == constants.ModelGroupProviderScopeAll {
		group.ProviderGuids = nil
	}
	if group.DisplayName == "" {
		group.DisplayName = group.GroupName
	}
	if group.QuotaMultiplier <= 0 {
		group.QuotaMultiplier = 1
	}
}

func normalizeProviderScope(scope string) string {
	if strings.EqualFold(strings.TrimSpace(scope), constants.ModelGroupProviderScopeSelected) {
		return constants.ModelGroupProviderScopeSelected
	}
	return constants.ModelGroupProviderScopeAll
}

func normalizeProviderGuids(providerGuids []string) []string {
	seen := make(map[string]struct{}, len(providerGuids))
	out := make([]string, 0, len(providerGuids))
	for _, providerGuid := range providerGuids {
		providerGuid = strings.TrimSpace(providerGuid)
		if providerGuid == "" {
			continue
		}
		if _, ok := seen[providerGuid]; ok {
			continue
		}
		seen[providerGuid] = struct{}{}
		out = append(out, providerGuid)
	}
	return out
}

func (s *ModelService) validateGroupProviders(providerGuids []string) error {
	if len(providerGuids) == 0 {
		return nil
	}
	var count int64
	if err := s.DB().Model(&domains.VendorMeta{}).Where("guid IN ?", providerGuids).Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(providerGuids)) {
		return errors.New("one or more selected providers do not exist")
	}
	return nil
}

func (s *ModelService) replaceGroupProviders(groupGuid string, scope string, providerGuids []string) error {
	if err := s.DB().Unscoped().Where("group_guid = ?", groupGuid).Delete(&domains.ModelGroupProvider{}).Error; err != nil {
		return err
	}
	if scope != constants.ModelGroupProviderScopeSelected {
		return nil
	}
	relations := make([]domains.ModelGroupProvider, 0, len(providerGuids))
	for index, providerGuid := range providerGuids {
		relations = append(relations, domains.ModelGroupProvider{
			GroupGuid:    groupGuid,
			ProviderGuid: providerGuid,
			Sort:         index,
		})
	}
	return s.DB().Create(&relations).Error
}

func (s *ModelService) fillModelGroupProviders(groups []domains.ModelGroup) error {
	if len(groups) == 0 {
		return nil
	}
	groupByGuid := make(map[string]*domains.ModelGroup, len(groups))
	groupGuids := make([]string, 0, len(groups))
	for index := range groups {
		groups[index].ProviderScope = normalizeProviderScope(groups[index].ProviderScope)
		groupByGuid[groups[index].Guid] = &groups[index]
		groupGuids = append(groupGuids, groups[index].Guid)
	}
	var relations []domains.ModelGroupProvider
	if err := s.GroupCrud.DB().Where("group_guid IN ?", groupGuids).Order("sort asc, id asc").Find(&relations).Error; err != nil {
		return err
	}
	for _, relation := range relations {
		if group := groupByGuid[relation.GroupGuid]; group != nil {
			group.ProviderGuids = append(group.ProviderGuids, relation.ProviderGuid)
			group.ProviderCount++
		}
	}
	return nil
}

func (s *ModelService) fillModelGroupProvider(group *domains.ModelGroup) error {
	groups := []domains.ModelGroup{*group}
	if err := s.fillModelGroupProviders(groups); err != nil {
		return err
	}
	*group = groups[0]
	return nil
}

func normalizeModelMeta(meta *domains.ModelMeta) {
	meta.Guid = strings.TrimSpace(meta.Guid)
	meta.ModelName = strings.TrimSpace(meta.ModelName)
	meta.DisplayName = strings.TrimSpace(meta.DisplayName)
	if len(meta.Groups) == 0 {
		meta.Groups = splitModelGroups(meta.Group)
	}
	meta.Groups = normalizeModelGroups(meta.Groups)
	meta.Group = strings.Join(meta.Groups, ",")
	meta.OwnedBy = strings.TrimSpace(meta.OwnedBy)
	meta.OfficialProvider = strings.TrimSpace(meta.OfficialProvider)
	meta.OfficialPriceUnit = strings.TrimSpace(meta.OfficialPriceUnit)
	meta.OfficialPricingRemark = strings.TrimSpace(meta.OfficialPricingRemark)
	meta.Remark = strings.TrimSpace(meta.Remark)
	if meta.OfficialPriceUnit == "" {
		meta.OfficialPriceUnit = "1M tokens"
	}
	if meta.OfficialInputPrice < 0 {
		meta.OfficialInputPrice = 0
	}
	if meta.OfficialOutputPrice < 0 {
		meta.OfficialOutputPrice = 0
	}
	if meta.OfficialCachePrice < 0 {
		meta.OfficialCachePrice = 0
	}
}

func fillModelMetaGroups(meta *domains.ModelMeta) {
	meta.FillGroups()
	if len(meta.Groups) == 0 {
		meta.Groups = []string{constants.DefaultGroup}
		meta.Group = constants.DefaultGroup
	}
}

func splitModelGroups(raw string) []string {
	return normalizeModelGroups(splitCSV(raw))
}

func normalizeModelGroups(groups []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(groups))
	for _, group := range groups {
		group = normalizeGroup(group)
		if group == "*" {
			return []string{"*"}
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		out = append(out, group)
	}
	if len(out) == 0 {
		return []string{constants.DefaultGroup}
	}
	return out
}

func modelGroupsContain(groups []string, group string) bool {
	group = normalizeGroup(group)
	return containsString(groups, "*") || containsString(groups, group)
}

func (s *ModelService) UpsertVendor(meta *domains.VendorMeta) error {
	if meta.Id == 0 {
		return createWithCrud(&s.VendorCrud, meta)
	}
	existing, err := s.VendorCrud.GetById(meta.Id)
	if err != nil {
		return err
	}
	if existing == nil {
		return errors.New("vendor not found")
	}
	meta.Guid = existing.Guid
	meta.CreateTime = existing.CreateTime
	meta.Creater = existing.Creater
	updating := *meta
	updating.Id = 0
	if err := createWithCrud(&s.VendorCrud, &updating); err != nil {
		return err
	}
	*meta = updating
	return nil
}

func (s *ModelService) ListVendors(includeDisabled bool) ([]domains.VendorMeta, error) {
	var vendors []domains.VendorMeta
	db := s.VendorCrud.DB().Order("sort desc, id desc")
	if !includeDisabled {
		db = db.Where("enabled = ?", true)
	}
	err := db.Find(&vendors).Error
	return vendors, err
}

func (s *ModelService) DeleteVendor(id uint) error {
	return deleteByIDWithCrud(&s.VendorCrud, id, "vendor not found")
}
