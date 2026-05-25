package domains

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

type Pricing struct {
	commonDomains.BaseDataEntity
	ModelName        string  `json:"modelName" gorm:"column:model_name;size:120;index:idx_pricing_model_group,unique;comment:模型名称，* 表示默认"`
	Group            string  `json:"group" gorm:"column:group_name;size:100;default:default;index:idx_pricing_model_group,unique;comment:分组，* 表示所有分组"`
	PromptMultiplier float64 `json:"promptMultiplier" gorm:"column:prompt_multiplier;default:1;comment:输入倍率"`
	OutputMultiplier float64 `json:"outputMultiplier" gorm:"column:output_multiplier;default:1;comment:输出倍率"`
	CacheMultiplier  float64 `json:"cacheMultiplier" gorm:"column:cache_multiplier;default:1;comment:缓存命中倍率"`
	QuotaMultiplier  float64 `json:"quotaMultiplier" gorm:"column:quota_multiplier;default:1;comment:总额度倍率"`
	Enabled          bool    `json:"enabled" gorm:"column:enabled;default:true;index;comment:启用"`
	Remark           string  `json:"remark" gorm:"column:remark;size:255;comment:备注"`
}

func (Pricing) TableName() string {
	return "nav_api_pricing"
}
