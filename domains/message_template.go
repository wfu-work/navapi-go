package domains

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

type MessageTemplate struct {
	commonDomains.BaseDataEntity
	Code        string `json:"code" gorm:"size:64;uniqueIndex;comment:模板编码"`
	Name        string `json:"name" gorm:"size:128;comment:模板名称"`
	Channel     string `json:"channel" gorm:"size:32;index;comment:发送渠道"`
	Subject     string `json:"subject" gorm:"size:255;comment:邮件主题"`
	Content     string `json:"content" gorm:"type:text;comment:模板内容"`
	Description string `json:"description" gorm:"size:512;comment:说明"`
	Status      int    `json:"status" gorm:"index;comment:状态"`
}

func (MessageTemplate) TableName() string {
	return "nav_api_message_templates"
}

func (s MessageTemplate) GetBaseData() commonDomains.BaseDataEntity {
	return s.BaseDataEntity
}
