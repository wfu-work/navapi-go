package services

import (
	"sort"
	"time"

	"navapi-go/domains"
	"navapi-go/dto"

	"github.com/wfu-work/nav-common-go-lib/global"
)

type ModelService struct{}

var ModelServiceApp = ModelService{}

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
	return global.NAV_DB.Save(meta).Error
}

func (s ModelService) ListMeta() ([]domains.ModelMeta, error) {
	var metas []domains.ModelMeta
	err := global.NAV_DB.Order("sort desc, id desc").Find(&metas).Error
	return metas, err
}

func (s ModelService) DeleteMeta(id uint) error {
	return global.NAV_DB.Delete(&domains.ModelMeta{}, id).Error
}

func (s ModelService) UpsertVendor(meta *domains.VendorMeta) error {
	return global.NAV_DB.Save(meta).Error
}

func (s ModelService) ListVendors(includeDisabled bool) ([]domains.VendorMeta, error) {
	var vendors []domains.VendorMeta
	db := global.NAV_DB.Order("sort desc, id desc")
	if !includeDisabled {
		db = db.Where("enabled = ?", true)
	}
	err := db.Find(&vendors).Error
	return vendors, err
}

func (s ModelService) DeleteVendor(id uint) error {
	return global.NAV_DB.Delete(&domains.VendorMeta{}, id).Error
}
