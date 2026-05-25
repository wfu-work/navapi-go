package domains

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

type Channel struct {
	commonDomains.BaseDataEntity
	Name           string `json:"name" gorm:"column:name;size:100;index;comment:渠道名称"`
	Type           string `json:"type" gorm:"column:type;size:40;index;comment:渠道类型"`
	Status         int    `json:"status" gorm:"column:status;default:1;index;comment:状态"`
	Key            string `json:"-" gorm:"column:key;type:text;comment:上游 API Key"`
	BaseURL        string `json:"baseUrl" gorm:"column:base_url;size:500;comment:上游 Base URL"`
	Models         string `json:"models" gorm:"column:models;type:text;comment:逗号分隔模型列表"`
	Group          string `json:"group" gorm:"column:group_name;size:100;default:default;index;comment:可用分组"`
	Tags           string `json:"tags" gorm:"column:tags;type:text;comment:逗号分隔标签"`
	Weight         int    `json:"weight" gorm:"column:weight;default:1;comment:权重"`
	Priority       int    `json:"priority" gorm:"column:priority;default:0;index;comment:优先级"`
	ModelMapping   string `json:"modelMapping" gorm:"column:model_mapping;type:text;comment:JSON 模型映射"`
	HeaderOverride string `json:"headerOverride" gorm:"column:header_override;type:text;comment:JSON 请求头覆盖"`
	ParamOverride  string `json:"paramOverride" gorm:"column:param_override;type:text;comment:JSON 参数覆盖"`
	UsedQuota      int64  `json:"usedQuota" gorm:"column:used_quota;default:0;comment:已用额度"`
	Balance        int64  `json:"balance" gorm:"column:balance;default:0;comment:余额快照"`
	TestModel      string `json:"testModel" gorm:"column:test_model;size:100;comment:测试模型"`
	TestTime       int64  `json:"testTime" gorm:"column:test_time;comment:测试时间"`
	ResponseTime   int64  `json:"responseTime" gorm:"column:response_time;comment:响应耗时毫秒"`
	DisabledReason string `json:"disabledReason" gorm:"column:disabled_reason;size:255;comment:自动禁用原因"`
	Remark         string `json:"remark" gorm:"column:remark;size:255;comment:备注"`
}

func (Channel) TableName() string {
	return "nav_api_channels"
}
