package constants

const (
	ProviderTypeOpenAI    = "openai"
	ProviderTypeAnthropic = "anthropic"
	ProviderTypeGemini    = "gemini"

	StatusEnabled  = 1
	StatusDisabled = 2

	DefaultGroup = "default"

	ModelGroupProviderScopeAll      = "all"
	ModelGroupProviderScopeSelected = "selected"

	ProviderRoutingFailover           = "failover"
	ProviderRoutingRoundRobin         = "round_robin"
	ProviderRoutingWeightedRoundRobin = "weighted_round_robin"
	ProviderRoutingLeastInflight      = "least_inflight"

	AdminUsername = "admin"

	ContextToken = "navapi_token"
	ContextModel = "navapi_model"
)
