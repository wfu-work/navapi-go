package services

import (
	"errors"
	"math"
	"strings"

	"navapi-go/domains"
	"navapi-go/dto"

	commonServices "github.com/wfu-work/nav-common-go-lib/services"
	"gorm.io/gorm"
)

type PricingService struct {
	commonServices.CrudService[domains.Pricing]
}

var PricingServiceApp = new(PricingService)

type QuotaCalculationDetail struct {
	Quota               int64   `json:"quota"`
	BillingMode         string  `json:"billingMode"`
	PricingMatched      bool    `json:"pricingMatched"`
	PricingModel        string  `json:"pricingModel,omitempty"`
	PricingGroup        string  `json:"pricingGroup,omitempty"`
	PromptMultiplier    float64 `json:"promptMultiplier"`
	OutputMultiplier    float64 `json:"outputMultiplier"`
	CacheMultiplier     float64 `json:"cacheMultiplier"`
	QuotaMultiplier     float64 `json:"quotaMultiplier"`
	GroupMultiplier     float64 `json:"groupMultiplier"`
	OfficialPricing     bool    `json:"officialPricing"`
	OfficialProvider    string  `json:"officialProvider,omitempty"`
	OfficialPriceUnit   string  `json:"officialPriceUnit,omitempty"`
	OfficialInputPrice  float64 `json:"officialInputPrice"`
	OfficialOutputPrice float64 `json:"officialOutputPrice"`
	OfficialCachePrice  float64 `json:"officialCachePrice"`
	PriceUnitTokens     float64 `json:"priceUnitTokens"`
	RawCost             float64 `json:"rawCost"`
	FinalCost           float64 `json:"finalCost"`
	RegularPromptTokens int64   `json:"regularPromptTokens"`
	CachedTokens        int64   `json:"cachedTokens"`
	CompletionTokens    int64   `json:"completionTokens"`
	FallbackQuota       int64   `json:"fallbackQuota"`
}

func (s *PricingService) WithDB(db *gorm.DB) *PricingService {
	cloned := *s
	cloned.CrudService = *s.CrudService.WithDB(db)
	return &cloned
}

func (s *PricingService) List(query dto.PageQuery) (dto.PageResult, error) {
	query.Normalize()
	var pricing []domains.Pricing
	var total int64
	db := s.DB()
	if db == nil {
		return dto.PageResult{}, errors.New("database is not initialized")
	}
	db = db.Model(&domains.Pricing{})
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

func (s *PricingService) PublicList() ([]domains.Pricing, error) {
	var pricing []domains.Pricing
	err := s.DB().Where("enabled = ?", true).Order("model_name asc, group_name asc").Find(&pricing).Error
	return pricing, err
}

func (s *PricingService) Upsert(pricing *domains.Pricing) error {
	normalizePricing(pricing)
	if pricing.Id == 0 {
		return createWithCrud(&s.CrudService, pricing)
	}
	existing, err := s.GetById(pricing.Id)
	if err != nil {
		return err
	}
	if existing == nil {
		return errors.New("pricing not found")
	}
	pricing.Guid = existing.Guid
	pricing.CreateTime = existing.CreateTime
	pricing.Creater = existing.Creater
	updating := *pricing
	updating.Id = 0
	if err := createWithCrud(&s.CrudService, &updating); err != nil {
		return err
	}
	*pricing = updating
	return nil
}

func (s *PricingService) Delete(id uint) error {
	return deleteByIDWithCrud(&s.CrudService, id, "pricing not found")
}

func (s *PricingService) CalculateQuota(modelName string, group string, usage dto.Usage, fallback int64) int64 {
	return s.CalculateQuotaDetail(modelName, group, usage, fallback).Quota
}

func (s *PricingService) CalculateQuotaDetail(modelName string, group string, usage dto.Usage, fallback int64) QuotaCalculationDetail {
	groupMultiplier := s.groupQuotaMultiplier(group)
	if groupMultiplier <= 0 {
		groupMultiplier = 1
	}
	detail := QuotaCalculationDetail{
		BillingMode:      "fallback",
		PromptMultiplier: 1,
		OutputMultiplier: 1,
		CacheMultiplier:  1,
		QuotaMultiplier:  1,
		GroupMultiplier:  groupMultiplier,
		CachedTokens:     usage.CachedTokens,
		CompletionTokens: usage.CompletionTokens,
		FallbackQuota:    fallback,
	}
	s.applyOfficialCost(&detail, modelName, group, usage)
	pricing := s.match(modelName, group)
	if pricing == nil {
		detail.Quota = int64(math.Ceil(float64(fallback) * groupMultiplier))
		return detail
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
	if !detail.OfficialPricing {
		detail.BillingMode = "token"
		detail.PricingMatched = true
		detail.PricingModel = pricing.ModelName
		detail.PricingGroup = pricing.Group
	}
	detail.PromptMultiplier = pricing.PromptMultiplier
	detail.OutputMultiplier = pricing.OutputMultiplier
	detail.CacheMultiplier = pricing.CacheMultiplier
	detail.QuotaMultiplier = pricing.QuotaMultiplier
	var quota float64
	if usage.PromptTokens > 0 || usage.CompletionTokens > 0 {
		cachedTokens := usage.CachedTokens
		if cachedTokens > usage.PromptTokens {
			cachedTokens = usage.PromptTokens
		}
		regularPromptTokens := usage.PromptTokens - cachedTokens
		detail.CachedTokens = cachedTokens
		detail.RegularPromptTokens = regularPromptTokens
		quota = float64(regularPromptTokens)*pricing.PromptMultiplier + float64(cachedTokens)*pricing.CacheMultiplier + float64(usage.CompletionTokens)*pricing.OutputMultiplier
	} else {
		detail.BillingMode = "fallback"
		quota = float64(fallback)
	}
	quota *= pricing.QuotaMultiplier
	quota *= groupMultiplier
	if quota <= 0 {
		detail.Quota = fallback
		return detail
	}
	detail.Quota = int64(math.Ceil(quota))
	return detail
}

func (s *PricingService) OfficialCostDetail(modelName string, group string, usage dto.Usage) QuotaCalculationDetail {
	groupMultiplier := s.groupQuotaMultiplier(group)
	if groupMultiplier <= 0 {
		groupMultiplier = 1
	}
	detail := QuotaCalculationDetail{
		BillingMode:      "official_price",
		PromptMultiplier: 1,
		OutputMultiplier: 1,
		CacheMultiplier:  1,
		QuotaMultiplier:  1,
		GroupMultiplier:  groupMultiplier,
		CachedTokens:     usage.CachedTokens,
		CompletionTokens: usage.CompletionTokens,
	}
	s.applyOfficialCost(&detail, modelName, group, usage)
	return detail
}

func (s *PricingService) groupQuotaMultiplier(group string) float64 {
	if s.DB() == nil {
		return ModelServiceApp.GroupQuotaMultiplier(group)
	}
	return ModelServiceApp.WithDB(s.DB()).GroupQuotaMultiplier(group)
}

func (s *PricingService) applyOfficialCost(detail *QuotaCalculationDetail, modelName string, group string, usage dto.Usage) {
	if detail == nil || s.DB() == nil {
		return
	}
	meta, ok := s.officialPricingMeta(modelName)
	if !ok {
		return
	}
	if usage.PromptTokens <= 0 && usage.CompletionTokens <= 0 {
		return
	}
	unitTokens := officialPriceUnitTokens(meta.OfficialPriceUnit)
	cachedTokens := usage.CachedTokens
	if cachedTokens > usage.PromptTokens {
		cachedTokens = usage.PromptTokens
	}
	if cachedTokens < 0 {
		cachedTokens = 0
	}
	regularPromptTokens := usage.PromptTokens - cachedTokens
	if regularPromptTokens < 0 {
		regularPromptTokens = 0
	}
	rawCost := float64(regularPromptTokens)*meta.OfficialInputPrice/unitTokens +
		float64(cachedTokens)*meta.OfficialCachePrice/unitTokens +
		float64(usage.CompletionTokens)*meta.OfficialOutputPrice/unitTokens
	if rawCost <= 0 {
		return
	}
	detail.BillingMode = "official_price"
	detail.PricingMatched = true
	detail.PricingModel = meta.ModelName
	detail.PricingGroup = normalizeGroup(group)
	detail.OfficialPricing = true
	detail.OfficialProvider = meta.OfficialProvider
	detail.OfficialPriceUnit = meta.OfficialPriceUnit
	detail.OfficialInputPrice = meta.OfficialInputPrice
	detail.OfficialOutputPrice = meta.OfficialOutputPrice
	detail.OfficialCachePrice = meta.OfficialCachePrice
	detail.PriceUnitTokens = unitTokens
	detail.RegularPromptTokens = regularPromptTokens
	detail.CachedTokens = cachedTokens
	detail.CompletionTokens = usage.CompletionTokens
	detail.RawCost = rawCost
	detail.FinalCost = rawCost * detail.GroupMultiplier
}

func (s *PricingService) officialPricingMeta(modelName string) (domains.ModelMeta, bool) {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return domains.ModelMeta{}, false
	}
	var meta domains.ModelMeta
	err := s.DB().Where("model_name = ? AND enabled = ?", modelName, true).First(&meta).Error
	if err != nil {
		return domains.ModelMeta{}, false
	}
	if meta.OfficialInputPrice <= 0 && meta.OfficialOutputPrice <= 0 && meta.OfficialCachePrice <= 0 {
		return domains.ModelMeta{}, false
	}
	if strings.TrimSpace(meta.OfficialPriceUnit) == "" {
		meta.OfficialPriceUnit = "1M tokens"
	}
	return meta, true
}

func officialPriceUnitTokens(unit string) float64 {
	unit = strings.ToLower(strings.TrimSpace(unit))
	switch {
	case strings.Contains(unit, "1k"):
		return 1000
	case unit == "token" || strings.Contains(unit, "/token"):
		return 1
	default:
		return 1000000
	}
}

func (s *PricingService) match(modelName string, group string) *domains.Pricing {
	group = normalizeGroup(group)
	candidates := []domains.Pricing{}
	err := s.DB().Where("enabled = ? AND model_name IN ? AND group_name IN ?", true, []string{modelName, "*"}, []string{group, "*", "default"}).
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
