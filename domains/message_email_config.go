package domains

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

type MessageEmailConfig struct {
	commonDomains.BaseDataEntity
	Name       string `json:"name" gorm:"size:128;comment:配置名称"`
	Host       string `json:"host" gorm:"size:255;comment:SMTP服务器"`
	Port       int    `json:"port" gorm:"comment:SMTP端口"`
	Username   string `json:"username" gorm:"size:255;comment:账号"`
	Password   string `json:"-" gorm:"size:512;comment:密码或授权码"`
	FromEmail  string `json:"fromEmail" gorm:"size:255;comment:发件邮箱"`
	FromName   string `json:"fromName" gorm:"size:128;comment:发件名称"`
	Encryption string `json:"encryption" gorm:"size:32;comment:加密方式 none/ssl/starttls"`
	IsDefault  bool   `json:"isDefault" gorm:"index;comment:默认配置"`
	Remark     string `json:"remark" gorm:"size:512;comment:备注"`
	Status     int    `json:"status" gorm:"index;comment:状态"`
}

func (MessageEmailConfig) TableName() string {
	return "nav_api_message_email_configs"
}

func (s MessageEmailConfig) GetBaseData() commonDomains.BaseDataEntity {
	return s.BaseDataEntity
}
