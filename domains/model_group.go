package domains

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

type ModelGroup struct {
	commonDomains.BaseDataEntity
	GroupName       string  `json:"groupName" gorm:"column:group_name;size:100;uniqueIndex;comment:分组标识"`
	DisplayName     string  `json:"displayName" gorm:"column:display_name;size:120;comment:展示名称"`
	QuotaMultiplier float64 `json:"quotaMultiplier" gorm:"column:quota_multiplier;default:1;comment:额度倍率"`
	Enabled         bool    `json:"enabled" gorm:"column:enabled;default:true;index;comment:启用"`
	Sort            int     `json:"sort" gorm:"column:sort;default:0;comment:排序"`
	Remark          string  `json:"remark" gorm:"column:remark;size:255;comment:备注"`
}

func (ModelGroup) TableName() string {
	return "nav_api_model_groups"
}
