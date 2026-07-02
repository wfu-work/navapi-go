package services

import (
	"errors"
	"sort"
	"time"

	"navapi-go/domains"
	"navapi-go/dto"

	commonServices "github.com/wfu-work/nav-common-go-lib/services"
	"gorm.io/gorm"
)

type ModelService struct {
	commonServices.CrudService[domains.ModelMeta]
	VendorCrud commonServices.CrudService[domains.VendorMeta]
}

var ModelServiceApp = new(ModelService)

func (s *ModelService) WithDB(db *gorm.DB) *ModelService {
	cloned := *s
	cloned.CrudService = *s.CrudService.WithDB(db)
	cloned.VendorCrud = *s.VendorCrud.WithDB(db)
	return &cloned
}

func (s ModelService) ListOpenAIModels() (dto.ModelListResponse, error) {
	models, err := ChannelServiceApp.ListEnabledModels()
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

func (s ModelService) UpsertMeta(meta *domains.ModelMeta) error {
	if meta.Id == 0 {
		return createWithCrud(&s.CrudService, meta)
	}
	existing, err := s.GetById(meta.Id)
	if err != nil {
		return err
	}
	if existing == nil {
		return errors.New("model not found")
	}
	meta.Guid = existing.Guid
	meta.CreateTime = existing.CreateTime
	meta.Creater = existing.Creater
	updating := *meta
	updating.Id = 0
	if err := createWithCrud(&s.CrudService, &updating); err != nil {
		return err
	}
	*meta = updating
	return nil
}

func (s ModelService) ListMeta() ([]domains.ModelMeta, error) {
	var metas []domains.ModelMeta
	err := s.DB().Order("sort desc, id desc").Find(&metas).Error
	return metas, err
}

func (s ModelService) DeleteMeta(id uint) error {
	return deleteByIDWithCrud(&s.CrudService, id, "model not found")
}

func (s ModelService) UpsertVendor(meta *domains.VendorMeta) error {
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

func (s ModelService) ListVendors(includeDisabled bool) ([]domains.VendorMeta, error) {
	var vendors []domains.VendorMeta
	db := s.VendorCrud.DB().Order("sort desc, id desc")
	if !includeDisabled {
		db = db.Where("enabled = ?", true)
	}
	err := db.Find(&vendors).Error
	return vendors, err
}

func (s ModelService) DeleteVendor(id uint) error {
	return deleteByIDWithCrud(&s.VendorCrud, id, "vendor not found")
}
