package domains

import (
	commonDomains "github.com/wfu-work/nav-common-go-lib/domains"
	"gorm.io/gorm"
)

const (
	ProbeLogTableName              = "nav_api_probe_logs"
	legacyServiceProbeLogTableName = "nav_api_service_probe_logs"
)

type ProbeLog struct {
	commonDomains.BaseDataEntity
	ModelName           string `json:"modelName" gorm:"column:model_name;size:120;index;comment:模型名称"`
	ProviderGuid        string `json:"providerGuid" gorm:"column:provider_guid;size:100;index;comment:上游服务商 GUID"`
	ProviderName        string `json:"providerName" gorm:"column:provider_name;size:120;comment:上游服务商名称"`
	ProviderType        string `json:"providerType" gorm:"column:provider_type;size:40;index;comment:上游类型"`
	Status              string `json:"status" gorm:"column:status;size:30;index;comment:success/error"`
	StatusCode          int    `json:"statusCode" gorm:"column:status_code;default:0;comment:上游状态码"`
	UseTimeMs           int64  `json:"useTimeMs" gorm:"column:use_time_ms;default:0;comment:总耗时毫秒"`
	FirstResponseTimeMs int64  `json:"firstResponseTimeMs" gorm:"column:first_response_time_ms;default:0;comment:首响应耗时毫秒"`
	PromptTokens        int64  `json:"promptTokens" gorm:"column:prompt_tokens;default:0;comment:输入 tokens"`
	CompletionTokens    int64  `json:"completionTokens" gorm:"column:completion_tokens;default:0;comment:输出 tokens"`
	Content             string `json:"content" gorm:"column:content;type:text;comment:摘要或错误内容"`
	UpstreamRequestID   string `json:"upstreamRequestId" gorm:"column:upstream_request_id;size:100;index;comment:上游请求 ID"`
	Other               string `json:"other" gorm:"column:other;type:text;comment:扩展信息 JSON"`
}

func (ProbeLog) TableName() string {
	return ProbeLogTableName
}

func migrateLegacyProbeLogTable(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if db.Migrator().HasTable(legacyServiceProbeLogTableName) && !db.Migrator().HasTable(ProbeLogTableName) {
		return db.Migrator().RenameTable(legacyServiceProbeLogTableName, ProbeLogTableName)
	}
	return nil
}
