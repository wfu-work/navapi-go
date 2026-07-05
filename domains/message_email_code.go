package domains

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

type MessageEmailCode struct {
	commonDomains.BaseDataEntity
	Email          string `json:"email" gorm:"size:255;index;comment:邮箱"`
	Scene          string `json:"scene" gorm:"size:64;index;comment:验证码场景"`
	Code           string `json:"-" gorm:"size:16;comment:验证码"`
	Status         string `json:"status" gorm:"size:32;index;comment:状态"`
	ExpiresTime    int64  `json:"expiresTime" gorm:"index;comment:过期时间毫秒"`
	UsedTime       int64  `json:"usedTime" gorm:"index;comment:使用时间毫秒"`
	SendRecordGuid string `json:"sendRecordGuid" gorm:"size:64;index;comment:发送记录GUID"`
	ClientIP       string `json:"clientIp" gorm:"size:80;index;comment:客户端IP"`
}

func (MessageEmailCode) TableName() string {
	return "nav_api_message_email_codes"
}

func (s MessageEmailCode) GetBaseData() commonDomains.BaseDataEntity {
	return s.BaseDataEntity
}
