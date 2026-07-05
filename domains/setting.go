package domains

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

type Setting struct {
	commonDomains.BaseDataEntity
	Key         string `gorm:"column:key;size:100;uniqueIndex" json:"key"`
	Value       string `gorm:"column:value;type:text" json:"value"`
	Description string `gorm:"column:description;size:500" json:"description"`
}

func (Setting) TableName() string {
	return "nginx_settings"
}
