package domains

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

type ChannelHealthLog struct {
	commonDomains.BaseDataEntity
	ChannelGuid  string `json:"channelGuid" gorm:"column:channel_guid;size:100;index;comment:渠道 GUID"`
	ChannelName  string `json:"channelName" gorm:"column:channel_name;size:100;index;comment:渠道名称"`
	ChannelID    uint   `json:"channelId" gorm:"column:channel_id;index;comment:渠道 ID"`
	OK           bool   `json:"ok" gorm:"column:ok;default:false;index;comment:是否健康"`
	Status       string `json:"status" gorm:"column:status;size:30;index;comment:检查状态"`
	StatusCode   int    `json:"statusCode" gorm:"column:status_code;default:0;comment:上游状态码"`
	ResponseTime int64  `json:"responseTime" gorm:"column:response_time;default:0;comment:响应耗时毫秒"`
	Models       string `json:"models" gorm:"column:models;type:text;comment:模型列表快照"`
	Error        string `json:"error" gorm:"column:error;type:text;comment:错误内容"`
	Trigger      string `json:"trigger" gorm:"column:trigger;size:40;index;comment:触发来源"`
	CheckedAt    int64  `json:"checkedAt" gorm:"column:checked_at;index;comment:检查时间秒"`
}

func (ChannelHealthLog) TableName() string {
	return "nav_api_channel_health_logs"
}
