package domains

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

type ModelGroup struct {
	commonDomains.BaseDataEntity
	GroupName       string                    `json:"groupName" gorm:"column:group_name;size:100;uniqueIndex;comment:分组标识"`
	DisplayName     string                    `json:"displayName" gorm:"column:display_name;size:120;comment:展示名称"`
	QuotaMultiplier float64                   `json:"quotaMultiplier" gorm:"column:quota_multiplier;default:1;comment:计费倍率"`
	ProviderScope   string                    `json:"providerScope" gorm:"column:provider_scope;size:20;default:all;index;comment:服务商范围"`
	RoutingStrategy string                    `json:"routingStrategy" gorm:"column:routing_strategy;size:40;default:failover;comment:服务商路由策略"`
	ProviderGuids   []string                  `json:"providerGuids,omitempty" gorm:"-"`
	ProviderRoutes  []ModelGroupProviderRoute `json:"providerRoutes,omitempty" gorm:"-"`
	ProviderCount   int                       `json:"providerCount,omitempty" gorm:"-"`
	Enabled         bool                      `json:"enabled" gorm:"column:enabled;default:true;index;comment:启用"`
	Sort            int                       `json:"sort" gorm:"column:sort;default:0;comment:排序"`
	Remark          string                    `json:"remark" gorm:"column:remark;size:255;comment:备注"`
}

func (ModelGroup) TableName() string {
	return "nav_api_model_groups"
}

type ModelGroupProvider struct {
	commonDomains.BaseDataEntity
	GroupGuid      string `json:"groupGuid" gorm:"column:group_guid;size:50;not null;index;uniqueIndex:uk_model_group_provider,priority:1;comment:模型分组GUID"`
	ProviderGuid   string `json:"providerGuid" gorm:"column:provider_guid;size:50;not null;index;uniqueIndex:uk_model_group_provider,priority:2;comment:服务商GUID"`
	Sort           int    `json:"sort" gorm:"column:sort;default:0;comment:排序"`
	Priority       int    `json:"priority" gorm:"column:priority;default:0;index;comment:路由优先级，数值越小越优先"`
	Weight         int    `json:"weight" gorm:"column:weight;default:100;comment:负载均衡权重"`
	MaxConcurrency int    `json:"maxConcurrency" gorm:"column:max_concurrency;default:0;comment:分组内最大并发，0表示不限制"`
	RoutingEnabled *bool  `json:"routingEnabled" gorm:"column:routing_enabled;default:true;index;comment:是否参与网关路由"`
}

func (ModelGroupProvider) TableName() string {
	return "nav_api_model_group_providers"
}

type ModelGroupProviderRoute struct {
	ProviderGuid   string `json:"providerGuid"`
	Sort           int    `json:"sort"`
	Priority       int    `json:"priority"`
	Weight         int    `json:"weight"`
	MaxConcurrency int    `json:"maxConcurrency"`
	RoutingEnabled bool   `json:"routingEnabled"`
}
