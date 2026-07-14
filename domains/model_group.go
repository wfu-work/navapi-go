package domains

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

type ModelGroup struct {
	commonDomains.BaseDataEntity
	GroupName       string   `json:"groupName" gorm:"column:group_name;size:100;uniqueIndex;comment:分组标识"`
	DisplayName     string   `json:"displayName" gorm:"column:display_name;size:120;comment:展示名称"`
	QuotaMultiplier float64  `json:"quotaMultiplier" gorm:"column:quota_multiplier;default:1;comment:计费倍率"`
	ProviderScope   string   `json:"providerScope" gorm:"column:provider_scope;size:20;default:all;index;comment:服务商范围"`
	ProviderGuids   []string `json:"providerGuids,omitempty" gorm:"-"`
	ProviderCount   int      `json:"providerCount,omitempty" gorm:"-"`
	Enabled         bool     `json:"enabled" gorm:"column:enabled;default:true;index;comment:启用"`
	Sort            int      `json:"sort" gorm:"column:sort;default:0;comment:排序"`
	Remark          string   `json:"remark" gorm:"column:remark;size:255;comment:备注"`
}

func (ModelGroup) TableName() string {
	return "nav_api_model_groups"
}

type ModelGroupProvider struct {
	commonDomains.BaseDataEntity
	GroupGuid    string `json:"groupGuid" gorm:"column:group_guid;size:50;not null;index;uniqueIndex:uk_model_group_provider,priority:1;comment:模型分组GUID"`
	ProviderGuid string `json:"providerGuid" gorm:"column:provider_guid;size:50;not null;index;uniqueIndex:uk_model_group_provider,priority:2;comment:服务商GUID"`
	Sort         int    `json:"sort" gorm:"column:sort;default:0;comment:排序"`
}

func (ModelGroupProvider) TableName() string {
	return "nav_api_model_group_providers"
}
