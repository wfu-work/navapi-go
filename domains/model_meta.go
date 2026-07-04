package domains

import (
	"strings"

	commonDomains "github.com/wfu-work/nav-common-go-lib/domains"
	"gorm.io/gorm"
)

type ModelMeta struct {
	commonDomains.BaseDataEntity
	ModelName             string   `json:"modelName" gorm:"column:model_name;size:120;uniqueIndex;comment:模型名称"`
	DisplayName           string   `json:"displayName" gorm:"column:display_name;size:120;comment:展示名称"`
	Group                 string   `json:"group" gorm:"column:group_name;size:255;default:default;index;comment:模型分组，逗号分隔"`
	Groups                []string `json:"groups,omitempty" gorm:"-"`
	OwnedBy               string   `json:"ownedBy" gorm:"column:owned_by;size:80;index;comment:供应商"`
	OfficialProvider      string   `json:"officialProvider" gorm:"column:official_provider;size:100;index;comment:官方提供商"`
	OfficialInputPrice    float64  `json:"officialInputPrice" gorm:"column:official_input_price;type:decimal(18,8);default:0;comment:官方输入价格"`
	OfficialOutputPrice   float64  `json:"officialOutputPrice" gorm:"column:official_output_price;type:decimal(18,8);default:0;comment:官方输出价格"`
	OfficialCachePrice    float64  `json:"officialCachePrice" gorm:"column:official_cache_price;type:decimal(18,8);default:0;comment:官方缓存价格"`
	OfficialPriceUnit     string   `json:"officialPriceUnit" gorm:"column:official_price_unit;size:40;default:1M tokens;comment:官方价格单位"`
	OfficialPricingRemark string   `json:"officialPricingRemark" gorm:"column:official_pricing_remark;size:255;comment:官方定价备注"`
	Enabled               bool     `json:"enabled" gorm:"column:enabled;default:true;index;comment:启用"`
	Sort                  int      `json:"sort" gorm:"column:sort;default:0;comment:排序"`
	ContextSize           int      `json:"contextSize" gorm:"column:context_size;default:0;comment:上下文长度"`
	Remark                string   `json:"remark" gorm:"column:remark;size:255;comment:备注"`
}

func (ModelMeta) TableName() string {
	return "nav_api_model_meta"
}

func (m *ModelMeta) AfterFind(_ *gorm.DB) error {
	m.FillGroups()
	return nil
}

func (m *ModelMeta) FillGroups() {
	groups := strings.FieldsFunc(m.Group, func(r rune) bool {
		return r == ',' || r == '，' || r == '\n'
	})
	out := make([]string, 0, len(groups))
	seen := map[string]struct{}{}
	for _, group := range groups {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		out = append(out, group)
	}
	if len(out) == 0 {
		out = []string{"default"}
	}
	m.Groups = out
}
