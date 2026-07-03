package services

import (
	"errors"
	"sort"
	"strings"
	"time"

	"navapi-go/constants"
	"navapi-go/domains"
	"navapi-go/dto"

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

func (s *ModelService) ListOpenAIModels() (dto.ModelListResponse, error) {
	models, err := ProviderServiceApp.ListEnabledModels()
	if err != nil {
		return dto.ModelListResponse{}, err
	}
	metas, err := s.ListMeta()
	if err != nil {
		return dto.ModelListResponse{}, err
	}
	metaByModel := map[string]domains.ModelMeta{}
	for _, meta := range metas {
		metaByModel[meta.ModelName] = meta
	}
	sort.Strings(models)
	data := make([]dto.ModelInfo, 0, len(models))
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
		data = append(data, dto.ModelInfo{
			ID:      model,
			Object:  "model",
			Created: now,
			OwnedBy: ownedBy,
		})
	}
	return dto.ModelListResponse{Object: "list", Data: data}, nil
}

func (s *ModelService) UpsertMeta(meta *domains.ModelMeta) error {
	meta.Guid = strings.TrimSpace(meta.Guid)
	meta.Group = normalizeGroup(meta.Group)
	if err := s.ValidateGroup(meta.Group); err != nil {
		return err
	}
	if meta.Guid == "" {
		meta.Id = 0
		return createWithCrud(&s.CrudService, meta)
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
	return reloadByGuidWithCrud(&s.CrudService, meta)
}

func (s *ModelService) ListMeta() ([]domains.ModelMeta, error) {
	var metas []domains.ModelMeta
	err := s.DB().Order("sort desc, id desc").Find(&metas).Error
	return metas, err
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
	err := db.Find(&groups).Error
	return groups, err
}

func (s *ModelService) UpsertGroup(group *domains.ModelGroup) error {
	normalizeModelGroup(group)
	if group.Guid == "" {
		group.Id = 0
		return createWithCrud(&s.GroupCrud, group)
	}
	existing, err := s.GroupCrud.GetByGuid(group.Guid)
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
	if err := s.GroupCrud.DB().Save(group).Error; err != nil {
		return err
	}
	return reloadByGuidWithCrud(&s.GroupCrud, group)
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
	if err := s.DB().Model(&domains.ModelMeta{}).Where("group_name = ?", group.GroupName).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("model group is in use")
	}
	return s.GroupCrud.DeleteByGuid(guid)
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
		Enabled:         true,
	}
	return createWithCrud(&s.GroupCrud, &group)
}

func (s *ModelService) ValidateGroup(group string) error {
	group = normalizeGroup(group)
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
		metaGroup := normalizeGroup(meta.Group)
		if metaGroup == group || metaGroup == "*" {
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
	group.GroupName = normalizeGroup(group.GroupName)
	group.DisplayName = strings.TrimSpace(group.DisplayName)
	group.Remark = strings.TrimSpace(group.Remark)
	if group.DisplayName == "" {
		group.DisplayName = group.GroupName
	}
	if group.QuotaMultiplier <= 0 {
		group.QuotaMultiplier = 1
	}
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
