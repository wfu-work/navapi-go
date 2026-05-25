package services

import (
	"math"

	"navapi-go/domains"
	"navapi-go/dto"

	"github.com/wfu-work/nav-common-go-lib/global"
)

type PricingService struct{}

var PricingServiceApp = PricingService{}

func (s PricingService) List(query dto.PageQuery) (dto.PageResult, error) {
	query.Normalize()
	var pricing []domains.Pricing
	var total int64
	db := global.NAV_DB.Model(&domains.Pricing{})
	if query.Q != "" {
		db = db.Where("model_name LIKE ? OR group_name LIKE ? OR remark LIKE ?", "%"+query.Q+"%", "%"+query.Q+"%", "%"+query.Q+"%")
	}
	if err := db.Count(&total).Error; err != nil {
		return dto.PageResult{}, err
	}
	if err := db.Order("model_name asc, group_name asc, id desc").Offset(query.Offset()).Limit(query.Size).Find(&pricing).Error; err != nil {
		return dto.PageResult{}, err
	}
	return dto.PageResult{List: pricing, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s PricingService) PublicList() ([]domains.Pricing, error) {
	var pricing []domains.Pricing
	err := global.NAV_DB.Where("enabled = ?", true).Order("model_name asc, group_name asc").Find(&pricing).Error
	return pricing, err
}

func (s PricingService) Upsert(pricing *domains.Pricing) error {
	normalizePricing(pricing)
	return global.NAV_DB.Save(pricing).Error
}

func (s PricingService) Delete(id uint) error {
	return global.NAV_DB.Delete(&domains.Pricing{}, id).Error
}

func (s PricingService) CalculateQuota(modelName string, group string, usage dto.Usage, fallback int64) int64 {
	pricing := s.match(modelName, group)
	if pricing == nil {
		return fallback
	}
	if pricing.QuotaMultiplier <= 0 {
		pricing.QuotaMultiplier = 1
	}
	if pricing.PromptMultiplier <= 0 {
		pricing.PromptMultiplier = 1
	}
	if pricing.OutputMultiplier <= 0 {
		pricing.OutputMultiplier = 1
	}
	if pricing.CacheMultiplier <= 0 {
		pricing.CacheMultiplier = 1
	}
	var quota float64
	if usage.PromptTokens > 0 || usage.CompletionTokens > 0 {
		cachedTokens := usage.CachedTokens
		if cachedTokens > usage.PromptTokens {
			cachedTokens = usage.PromptTokens
		}
		regularPromptTokens := usage.PromptTokens - cachedTokens
		quota = float64(regularPromptTokens)*pricing.PromptMultiplier + float64(cachedTokens)*pricing.CacheMultiplier + float64(usage.CompletionTokens)*pricing.OutputMultiplier
	} else {
		quota = float64(fallback)
	}
	quota *= pricing.QuotaMultiplier
	if quota <= 0 {
		return fallback
	}
	return int64(math.Ceil(quota))
}

func (s PricingService) match(modelName string, group string) *domains.Pricing {
	group = normalizeGroup(group)
	candidates := []domains.Pricing{}
	err := global.NAV_DB.Where("enabled = ? AND model_name IN ? AND group_name IN ?", true, []string{modelName, "*"}, []string{group, "*", "default"}).
		Find(&candidates).Error
	if err != nil || len(candidates) == 0 {
		return nil
	}
	for _, item := range candidates {
		if item.ModelName == modelName && item.Group == group {
			return &item
		}
	}
	for _, item := range candidates {
		if item.ModelName == modelName && item.Group == "default" {
			return &item
		}
	}
	for _, item := range candidates {
		if item.ModelName == modelName && item.Group == "*" {
			return &item
		}
	}
	for _, item := range candidates {
		if item.ModelName == "*" && item.Group == group {
			return &item
		}
	}
	for _, item := range candidates {
		if item.ModelName == "*" && item.Group == "default" {
			return &item
		}
	}
	for _, item := range candidates {
		if item.ModelName == "*" && item.Group == "*" {
			return &item
		}
	}
	return nil
}

func normalizePricing(pricing *domains.Pricing) {
	if pricing.ModelName == "" {
		pricing.ModelName = "*"
	}
	if pricing.Group == "" {
		pricing.Group = "default"
	}
	if pricing.PromptMultiplier <= 0 {
		pricing.PromptMultiplier = 1
	}
	if pricing.OutputMultiplier <= 0 {
		pricing.OutputMultiplier = 1
	}
	if pricing.CacheMultiplier <= 0 {
		pricing.CacheMultiplier = 1
	}
	if pricing.QuotaMultiplier <= 0 {
		pricing.QuotaMultiplier = 1
	}
}
