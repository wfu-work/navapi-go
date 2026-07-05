package domains

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

type MessageSendRecord struct {
	commonDomains.BaseDataEntity
	BatchGuid      string `json:"batchGuid" gorm:"size:64;index;comment:发送批次ID"`
	Channel        string `json:"channel" gorm:"size:32;index;comment:发送渠道"`
	TemplateCode   string `json:"templateCode" gorm:"size:64;index;comment:模板编码"`
	TemplateName   string `json:"templateName" gorm:"size:128;comment:模板名称"`
	Subject        string `json:"subject" gorm:"size:255;comment:邮件主题"`
	RecipientEmail string `json:"recipientEmail" gorm:"size:255;index;comment:接收邮箱"`
	FromEmail      string `json:"fromEmail" gorm:"size:255;comment:发件邮箱"`
	FromName       string `json:"fromName" gorm:"size:128;comment:发件名称"`
	HTMLContent    string `json:"-" gorm:"type:text;comment:邮件HTML内容"`
	SendStatus     string `json:"sendStatus" gorm:"size:32;index;comment:发送状态"`
	ReceiveStatus  string `json:"receiveStatus" gorm:"size:32;index;comment:接收状态"`
	RetryCount     int    `json:"retryCount" gorm:"comment:重试次数"`
	MaxRetries     int    `json:"maxRetries" gorm:"comment:最大重试次数"`
	ErrorMessage   string `json:"errorMessage" gorm:"type:text;comment:错误信息"`
	LastSendTime   int64  `json:"lastSendTime" gorm:"index;comment:最后发送时间"`
	NextRetryTime  int64  `json:"nextRetryTime" gorm:"index;comment:下次重试时间"`
	SuccessTime    int64  `json:"successTime" gorm:"index;comment:成功时间"`
}

func (MessageSendRecord) TableName() string {
	return "nav_api_message_send_records"
}

func (s MessageSendRecord) GetBaseData() commonDomains.BaseDataEntity {
	return s.BaseDataEntity
}
