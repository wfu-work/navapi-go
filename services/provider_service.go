package services

import (
	"errors"
	"strings"

	"navapi-go/constants"
	"navapi-go/domains"
	"navapi-go/dto"

	commonServices "github.com/wfu-work/nav-common-go-lib/services"
	"gorm.io/gorm"
)

type ProviderService struct {
	commonServices.CrudService[domains.VendorMeta]
}

var ProviderServiceApp = new(ProviderService)

func (s *ProviderService) WithDB(db *gorm.DB) *ProviderService {
	cloned := *s
	cloned.CrudService = *s.CrudService.WithDB(db)
	return &cloned
}

type ProviderChannelRequest struct {
	Name     string `json:"name"`
	Group    string `json:"group"`
	Tags     string `json:"tags"`
	Weight   int    `json:"weight"`
	Priority int    `json:"priority"`
	Key      string `json:"key"`
}

func (s ProviderService) List(query dto.PageQuery) (dto.PageResult, error) {
	query.Normalize()
	var providers []domains.VendorMeta
	var total int64
	db := s.DB()
	if db == nil {
		return dto.PageResult{}, errors.New("database is not initialized")
	}
	db = db.Model(&domains.VendorMeta{})
	if query.Q != "" {
		db = db.Where("vendor_name LIKE ? OR display_name LIKE ? OR type LIKE ? OR base_url LIKE ? OR remark LIKE ?", "%"+query.Q+"%", "%"+query.Q+"%", "%"+query.Q+"%", "%"+query.Q+"%", "%"+query.Q+"%")
	}
	if err := db.Count(&total).Error; err != nil {
		return dto.PageResult{}, err
	}
	if err := db.Order("sort desc, id desc").Offset(query.Offset()).Limit(query.Size).Find(&providers).Error; err != nil {
		return dto.PageResult{}, err
	}
	return dto.PageResult{List: providers, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s ProviderService) GetByID(id uint) (*domains.VendorMeta, error) {
	if id == 0 {
		return nil, errors.New("id is required")
	}
	provider, err := s.GetById(id)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, errors.New("provider not found")
	}
	return provider, nil
}

// Save normalizes provider defaults and validates JSON override fields before
// storing the upstream template used to create channels.
func (s ProviderService) Save(provider *domains.VendorMeta) error {
	if strings.TrimSpace(provider.VendorName) == "" {
		return errors.New("provider name is required")
	}
	if strings.TrimSpace(provider.Type) == "" {
		provider.Type = constants.ChannelTypeOpenAI
	}
	if provider.Id == 0 && !provider.Enabled {
		provider.Enabled = true
	}
	if err := validateOptionalJSONObject(provider.ModelMapping, "modelMapping"); err != nil {
		return err
	}
	if err := validateOptionalJSONObject(provider.HeaderOverride, "headerOverride"); err != nil {
		return err
	}
	if err := validateOptionalJSONObject(provider.ParamOverride, "paramOverride"); err != nil {
		return err
	}
	if provider.Id == 0 {
		return createWithCrud(&s.CrudService, provider)
	}
	existing, err := s.GetByID(provider.Id)
	if err != nil {
		return err
	}
	provider.Guid = existing.Guid
	provider.CreateTime = existing.CreateTime
	provider.Creater = existing.Creater
	updating := *provider
	updating.Id = 0
	if err := createWithCrud(&s.CrudService, &updating); err != nil {
		return err
	}
	*provider = updating
	return nil
}

func (s ProviderService) Delete(id uint) error {
	return deleteByIDWithCrud(&s.CrudService, id, "provider not found")
}

func (s ProviderService) GetKey(id uint) (string, error) {
	provider, err := s.GetByID(id)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(provider.Key) == "" {
		return "", errors.New("provider key is empty")
	}
	return provider.Key, nil
}

func (s ProviderService) SetKey(id uint, key string) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("provider key is required")
	}
	provider, err := s.GetByID(id)
	if err != nil {
		return err
	}
	return s.Update(*provider, "key", key)
}

// CreateChannel materializes a provider template into a runnable relay channel.
// The caller can override the runtime key without changing the provider default.
func (s ProviderService) CreateChannel(id uint, req ProviderChannelRequest) (*domains.Channel, error) {
	provider, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	key := provider.Key
	if strings.TrimSpace(req.Key) != "" {
		key = req.Key
	}
	if strings.TrimSpace(key) == "" {
		return nil, errors.New("channel key is required")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = provider.DisplayName
	}
	if name == "" {
		name = provider.VendorName
	}
	channel := &domains.Channel{
		Name:           name,
		Type:           provider.Type,
		Status:         constants.StatusEnabled,
		Key:            key,
		BaseURL:        provider.BaseURL,
		Models:         provider.Models,
		Group:          normalizeGroup(req.Group),
		Tags:           req.Tags,
		Weight:         req.Weight,
		Priority:       req.Priority,
		ModelMapping:   provider.ModelMapping,
		HeaderOverride: provider.HeaderOverride,
		ParamOverride:  provider.ParamOverride,
		Remark:         provider.Remark,
	}
	if err := ChannelServiceApp.Create(channel); err != nil {
		return nil, err
	}
	channel.Key = ""
	return channel, nil
}
