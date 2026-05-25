package domains

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

type ModelMeta struct {
	commonDomains.BaseDataEntity
	ModelName   string `json:"modelName" gorm:"column:model_name;size:120;uniqueIndex;comment:模型名称"`
	DisplayName string `json:"displayName" gorm:"column:display_name;size:120;comment:展示名称"`
	OwnedBy     string `json:"ownedBy" gorm:"column:owned_by;size:80;index;comment:供应商"`
	Enabled     bool   `json:"enabled" gorm:"column:enabled;default:true;index;comment:启用"`
	Sort        int    `json:"sort" gorm:"column:sort;default:0;comment:排序"`
	ContextSize int    `json:"contextSize" gorm:"column:context_size;default:0;comment:上下文长度"`
	Remark      string `json:"remark" gorm:"column:remark;size:255;comment:备注"`
}

func (ModelMeta) TableName() string {
	return "nav_api_model_meta"
}
