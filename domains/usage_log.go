package domains

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

const UsageLogSourceUser = "user"

type UsageLog struct {
	commonDomains.BaseDataEntity
	UserGuid             string  `json:"userGuid" gorm:"column:user_guid;size:100;index;comment:用户 GUID"`
	Username             string  `json:"username" gorm:"column:username;size:100;index;comment:用户名"`
	Email                string  `json:"email,omitempty" gorm:"-"`
	TokenGuid            string  `json:"tokenGuid" gorm:"column:token_guid;size:100;index;comment:令牌 GUID"`
	TokenName            string  `json:"tokenName" gorm:"column:token_name;size:100;comment:令牌名称"`
	ProviderGuid         string  `json:"providerGuid" gorm:"column:channel_guid;size:100;index;comment:上游服务商 GUID"`
	ProviderName         string  `json:"providerName" gorm:"column:channel_name;size:100;comment:上游服务商名称"`
	ModelName            string  `json:"modelName" gorm:"column:model_name;size:100;index;comment:模型名称"`
	Quota                int64   `json:"quota" gorm:"column:quota;default:0;comment:Token 用量"`
	Cost                 float64 `json:"cost" gorm:"column:cost;type:decimal(20,10);default:0;comment:消耗金额"`
	PromptTokens         int64   `json:"promptTokens" gorm:"column:prompt_tokens;default:0;comment:输入 tokens"`
	CompletionTokens     int64   `json:"completionTokens" gorm:"column:completion_tokens;default:0;comment:输出 tokens"`
	UseTimeMs            int64   `json:"useTimeMs" gorm:"column:use_time_ms;default:0;comment:耗时毫秒"`
	FirstResponseTimeMs  int64   `json:"firstResponseTimeMs" gorm:"column:first_response_time_ms;default:0;comment:首响应耗时毫秒"`
	RequestBodyBytes     int64   `json:"requestBodyBytes" gorm:"column:request_body_bytes;default:0;comment:上游请求体字节数"`
	DNSLookupTimeMs      int64   `json:"dnsLookupTimeMs" gorm:"column:dns_lookup_time_ms;default:0;comment:DNS查询耗时毫秒"`
	ConnectTimeMs        int64   `json:"connectTimeMs" gorm:"column:connect_time_ms;default:0;comment:TCP连接耗时毫秒"`
	TLSHandshakeTimeMs   int64   `json:"tlsHandshakeTimeMs" gorm:"column:tls_handshake_time_ms;default:0;comment:TLS握手耗时毫秒"`
	RequestWriteTimeMs   int64   `json:"requestWriteTimeMs" gorm:"column:request_write_time_ms;default:0;comment:请求发送完成耗时毫秒"`
	ResponseHeaderTimeMs int64   `json:"responseHeaderTimeMs" gorm:"column:response_header_time_ms;default:0;comment:上游响应头耗时毫秒"`
	UpstreamTotalTimeMs  int64   `json:"upstreamTotalTimeMs" gorm:"column:upstream_total_time_ms;default:0;comment:最终上游请求总耗时毫秒"`
	ConnectionReused     bool    `json:"connectionReused" gorm:"column:connection_reused;default:false;comment:是否复用上游连接"`
	AttemptCount         int     `json:"attemptCount" gorm:"column:attempt_count;default:0;comment:上游转发尝试次数"`
	IsStream             bool    `json:"isStream" gorm:"column:is_stream;default:false;comment:是否流式"`
	Status               string  `json:"status" gorm:"column:status;size:30;index;comment:success/error"`
	Content              string  `json:"content" gorm:"column:content;type:text;comment:摘要或错误内容"`
	RequestID            string  `json:"requestId" gorm:"column:request_id;size:100;index;comment:请求 ID"`
	UpstreamRequestID    string  `json:"upstreamRequestId" gorm:"column:upstream_request_id;size:100;index;comment:上游请求 ID"`
	ClientIP             string  `json:"clientIp" gorm:"column:client_ip;size:80;index;comment:客户端 IP"`
	Source               string  `json:"source" gorm:"column:source;size:30;index;default:user;comment:日志来源"`
	Other                string  `json:"other" gorm:"column:other;type:text;comment:扩展信息 JSON"`
}

func (UsageLog) TableName() string {
	return "nav_api_usage_logs"
}
