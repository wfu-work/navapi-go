package domains

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

type VendorMeta struct {
	commonDomains.BaseDataEntity
	VendorName           string   `json:"vendorName" gorm:"column:vendor_name;size:80;uniqueIndex;comment:供应商名称"`
	DisplayName          string   `json:"displayName" gorm:"column:display_name;size:120;comment:展示名称"`
	Type                 string   `json:"type" gorm:"column:type;size:40;default:openai;index;comment:上游类型"`
	LogoURL              string   `json:"logoUrl" gorm:"column:logo_url;size:500;comment:Logo URL"`
	BaseURL              string   `json:"baseUrl" gorm:"column:base_url;size:500;comment:默认 Base URL"`
	Key                  string   `json:"-" gorm:"column:key;type:text;comment:上游 API Key"`
	Models               string   `json:"models" gorm:"column:models;type:text;comment:逗号分隔模型列表"`
	EndpointCapabilities string   `json:"endpointCapabilities" gorm:"column:endpoint_capabilities;type:text;comment:逗号分隔端点能力"`
	ModelOverride        string   `json:"modelOverride" gorm:"column:model_override;size:120;comment:上游模型覆盖"`
	QuotaModelWhitelist  string   `json:"quotaModelWhitelist" gorm:"column:quota_model_whitelist;type:text;comment:余额模型白名单"`
	ModelMapping         string   `json:"modelMapping" gorm:"column:model_mapping;type:text;comment:JSON 模型映射"`
	HeaderOverride       string   `json:"headerOverride" gorm:"column:header_override;type:text;comment:JSON 请求头覆盖"`
	ParamOverride        string   `json:"paramOverride" gorm:"column:param_override;type:text;comment:JSON 参数覆盖"`
	ProxyEnabled         bool     `json:"proxyEnabled" gorm:"column:proxy_enabled;default:false;comment:启用代理"`
	ProxyType            string   `json:"proxyType" gorm:"column:proxy_type;size:20;default:http;comment:代理类型"`
	ProxyURL             string   `json:"proxyUrl" gorm:"column:proxy_url;size:500;comment:代理地址"`
	ProxyUsername        string   `json:"proxyUsername" gorm:"column:proxy_username;size:120;comment:代理用户名"`
	ProxyPassword        string   `json:"proxyPassword,omitempty" gorm:"column:proxy_password;type:text;comment:代理密码"`
	BalanceCheckEnabled  bool     `json:"balanceCheckEnabled" gorm:"column:balance_check_enabled;default:false;comment:启用余额检测"`
	BalanceTemplate      string   `json:"balanceTemplate" gorm:"column:balance_template;size:40;default:generic;comment:余额查询模板"`
	BalanceBaseURL       string   `json:"balanceBaseUrl" gorm:"column:balance_base_url;size:500;comment:余额接口基础地址"`
	BalanceAccessToken   string   `json:"balanceAccessToken" gorm:"column:balance_access_token;type:text;comment:余额查询 Access Token"`
	BalanceUserID        string   `json:"balanceUserId" gorm:"column:balance_user_id;size:120;comment:余额查询用户 ID"`
	BalanceCustomPath    string   `json:"balanceCustomPath" gorm:"column:balance_custom_path;size:255;comment:自定义余额查询路径"`
	BalanceAuthType      string   `json:"balanceAuthType" gorm:"column:balance_auth_type;size:40;comment:余额查询认证类型"`
	BalanceRemainingPath string   `json:"balanceRemainingPath" gorm:"column:balance_remaining_path;size:255;comment:剩余余额 JSON 路径"`
	BalanceMultiplier    float64  `json:"balanceMultiplier" gorm:"column:balance_multiplier;default:1;comment:余额倍率"`
	BalanceUnit          string   `json:"balanceUnit" gorm:"column:balance_unit;size:40;comment:余额单位"`
	BalanceTotalPath     string   `json:"balanceTotalPath" gorm:"column:balance_total_path;size:255;comment:总余额 JSON 路径"`
	BalanceUsedPath      string   `json:"balanceUsedPath" gorm:"column:balance_used_path;size:255;comment:已用余额 JSON 路径"`
	BalancePlanPath      string   `json:"balancePlanPath" gorm:"column:balance_plan_path;size:255;comment:套餐 JSON 路径"`
	BalanceValidPath     string   `json:"balanceValidPath" gorm:"column:balance_valid_path;size:255;comment:有效状态 JSON 路径"`
	BalanceErrorPath     string   `json:"balanceErrorPath" gorm:"column:balance_error_path;size:255;comment:错误信息 JSON 路径"`
	BalanceQueryOK       bool     `json:"balanceQueryOk" gorm:"column:balance_query_ok;default:false;comment:最近余额查询是否成功"`
	BalanceRemaining     *float64 `json:"balanceRemaining,omitempty" gorm:"column:balance_remaining;type:decimal(30,10);comment:最近成功查询的剩余余额"`
	BalanceTotal         *float64 `json:"balanceTotal,omitempty" gorm:"column:balance_total;type:decimal(30,10);comment:最近成功查询的总余额"`
	BalanceUsed          *float64 `json:"balanceUsed,omitempty" gorm:"column:balance_used;type:decimal(30,10);comment:最近成功查询的已用余额"`
	BalanceSnapshotUnit  string   `json:"balanceSnapshotUnit,omitempty" gorm:"column:balance_snapshot_unit;size:40;comment:余额快照单位"`
	BalanceSnapshotPlan  string   `json:"balanceSnapshotPlan,omitempty" gorm:"column:balance_snapshot_plan;size:255;comment:余额快照套餐"`
	BalanceQueryMessage  string   `json:"balanceQueryMessage,omitempty" gorm:"column:balance_query_message;size:1000;comment:最近余额查询信息"`
	BalanceStatusCode    int      `json:"balanceStatusCode,omitempty" gorm:"column:balance_status_code;default:0;comment:最近余额查询 HTTP 状态码"`
	BalanceResponseTime  int64    `json:"balanceResponseTimeMs,omitempty" gorm:"column:balance_response_time_ms;default:0;comment:最近余额查询耗时毫秒"`
	BalanceCheckedTime   int64    `json:"balanceCheckedTime,omitempty" gorm:"column:balance_checked_time;default:0;index;comment:最近余额查询时间"`
	BalanceSuccessTime   int64    `json:"balanceSuccessTime,omitempty" gorm:"column:balance_success_time;default:0;comment:最近余额查询成功时间"`
	Website              string   `json:"website" gorm:"column:website;size:500;comment:官网"`
	Enabled              bool     `json:"enabled" gorm:"column:enabled;default:true;index;comment:启用"`
	Sort                 int      `json:"sort" gorm:"column:sort;default:0;comment:排序"`
	Remark               string   `json:"remark" gorm:"column:remark;size:255;comment:备注"`
}

func (VendorMeta) TableName() string {
	return "nav_api_vendor_meta"
}
