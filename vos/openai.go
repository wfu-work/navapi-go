package vos

type OpenAIChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type ModelRequest struct {
	Model     string `json:"model"`
	ModelName string `json:"model_name"`
	Stream    bool   `json:"stream"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type OpenAIErrorResponse struct {
	Error OpenAIError `json:"error"`
}

type OpenAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

type ModelListResponse struct {
	Object string      `json:"object"`
	Data   []ModelInfo `json:"data"`
}

type ModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type Usage struct {
	PromptTokens           int64             `json:"prompt_tokens"`
	CompletionTokens       int64             `json:"completion_tokens"`
	TotalTokens            int64             `json:"total_tokens"`
	InputTokens            int64             `json:"input_tokens,omitempty"`
	OutputTokens           int64             `json:"output_tokens,omitempty"`
	PromptTokensDetails    TokenUsageDetails `json:"prompt_tokens_details,omitempty"`
	InputTokensDetails     TokenUsageDetails `json:"input_tokens_details,omitempty"`
	CompletionTokenDetails TokenUsageDetails `json:"completion_tokens_details,omitempty"`
	OutputTokensDetails    TokenUsageDetails `json:"output_tokens_details,omitempty"`
	CachedTokens           int64             `json:"cached_tokens,omitempty"`
}

type TokenUsageDetails struct {
	CachedTokens int64 `json:"cached_tokens,omitempty"`
}
