package domains

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

type VendorMeta struct {
	commonDomains.BaseDataEntity
	VendorName     string `json:"vendorName" gorm:"column:vendor_name;size:80;uniqueIndex;comment:供应商名称"`
	DisplayName    string `json:"displayName" gorm:"column:display_name;size:120;comment:展示名称"`
	Type           string `json:"type" gorm:"column:type;size:40;default:openai;index;comment:渠道类型"`
	LogoURL        string `json:"logoUrl" gorm:"column:logo_url;size:500;comment:Logo URL"`
	BaseURL        string `json:"baseUrl" gorm:"column:base_url;size:500;comment:默认 Base URL"`
	Key            string `json:"-" gorm:"column:key;type:text;comment:上游 API Key"`
	Models         string `json:"models" gorm:"column:models;type:text;comment:逗号分隔模型列表"`
	ModelMapping   string `json:"modelMapping" gorm:"column:model_mapping;type:text;comment:JSON 模型映射"`
	HeaderOverride string `json:"headerOverride" gorm:"column:header_override;type:text;comment:JSON 请求头覆盖"`
	ParamOverride  string `json:"paramOverride" gorm:"column:param_override;type:text;comment:JSON 参数覆盖"`
	Website        string `json:"website" gorm:"column:website;size:500;comment:官网"`
	Enabled        bool   `json:"enabled" gorm:"column:enabled;default:true;index;comment:启用"`
	Sort           int    `json:"sort" gorm:"column:sort;default:0;comment:排序"`
	Remark         string `json:"remark" gorm:"column:remark;size:255;comment:备注"`
}

func (VendorMeta) TableName() string {
	return "nav_api_vendor_meta"
}
