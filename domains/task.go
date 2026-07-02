package domains

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

type Task struct {
	commonDomains.BaseDataEntity
	TaskID       string `json:"taskId" gorm:"column:task_id;size:100;index;comment:公开任务 ID"`
	Platform     string `json:"platform" gorm:"column:platform;size:40;index;comment:平台"`
	UserGuid     string `json:"userGuid" gorm:"column:user_guid;size:100;index;comment:用户 GUID"`
	Group        string `json:"group" gorm:"column:group_name;size:100;index;comment:分组"`
	ProviderGuid string `json:"providerGuid" gorm:"column:channel_guid;size:100;index;comment:上游服务商 GUID"`
	ModelName    string `json:"modelName" gorm:"column:model_name;size:100;index;comment:模型"`
	Quota        int64  `json:"quota" gorm:"column:quota;default:0;comment:额度"`
	Action       string `json:"action" gorm:"column:action;size:60;index;comment:动作"`
	Status       string `json:"status" gorm:"column:status;size:30;index;comment:状态"`
	FailReason   string `json:"failReason" gorm:"column:fail_reason;type:text;comment:失败原因"`
	Progress     string `json:"progress" gorm:"column:progress;size:30;comment:进度"`
	Data         string `json:"data" gorm:"column:data;type:text;comment:任务数据 JSON"`
	PrivateData  string `json:"-" gorm:"column:private_data;type:text;comment:内部数据 JSON"`
}

func (Task) TableName() string {
	return "nav_api_tasks"
}
