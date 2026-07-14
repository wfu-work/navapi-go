package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"navapi-go/constants"
	"navapi-go/domains"
	"navapi-go/vos"
)

const (
	providerDebugTimeout          = 60 * time.Second
	providerDebugDefaultMaxTokens = 64
	providerDebugMaxTokens        = 4096
)

type ProviderChatDebugInput struct {
	Provider  domains.VendorMeta `json:"provider"`
	Key       string             `json:"key"`
	Model     string             `json:"model"`
	Prompt    string             `json:"prompt"`
	MaxTokens int                `json:"maxTokens"`
}

type ProviderChatDebugResult struct {
	OK                  bool      `json:"ok"`
	StatusCode          int       `json:"statusCode,omitempty"`
	ResponseTimeMs      int64     `json:"responseTimeMs"`
	FirstResponseTimeMs int64     `json:"firstResponseTimeMs"`
	ProviderType        string    `json:"providerType"`
	Model               string    `json:"model"`
	UpstreamModel       string    `json:"upstreamModel"`
	RequestPath         string    `json:"requestPath"`
	Content             string    `json:"content,omitempty"`
	Message             string    `json:"message"`
	Usage               vos.Usage `json:"usage"`
	Response            any       `json:"response,omitempty"`
}

func (s *ProviderService) DebugChat(ctx context.Context, input ProviderChatDebugInput) (*ProviderChatDebugResult, error) {
	provider := input.Provider
	provider.Type = strings.ToLower(strings.TrimSpace(provider.Type))
	if provider.Type == "" {
		provider.Type = constants.ProviderTypeOpenAI
	}
	provider.BaseURL = strings.TrimSpace(provider.BaseURL)
	provider.ModelOverride = strings.TrimSpace(provider.ModelOverride)
	provider.ModelMapping = strings.TrimSpace(provider.ModelMapping)
	provider.HeaderOverride = strings.TrimSpace(provider.HeaderOverride)
	provider.ParamOverride = strings.TrimSpace(provider.ParamOverride)
	provider.Key = strings.TrimSpace(input.Key)
	if err := s.hydrateProviderDebugSecrets(&provider); err != nil {
		return nil, err
	}
	if provider.BaseURL == "" {
		return nil, errors.New("base url is required")
	}
	if strings.TrimSpace(provider.Key) == "" {
		return nil, errors.New("provider key is required")
	}
	if err := validateOptionalJSONObject(provider.ModelMapping, "modelMapping"); err != nil {
		return nil, err
	}
	if err := validateOptionalJSONObject(provider.HeaderOverride, "headerOverride"); err != nil {
		return nil, err
	}
	if err := validateOptionalJSONObject(provider.ParamOverride, "paramOverride"); err != nil {
		return nil, err
	}
	normalizeProviderProxyConfig(&provider)
	if err := validateProviderProxyConfig(&provider); err != nil {
		return nil, err
	}

	model := strings.TrimSpace(input.Model)
	if model == "" {
		return nil, errors.New("model is required")
	}
	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" {
		return nil, errors.New("prompt is required")
	}
	maxTokens := input.MaxTokens
	if maxTokens <= 0 {
		maxTokens = providerDebugDefaultMaxTokens
	}
	if maxTokens > providerDebugMaxTokens {
		maxTokens = providerDebugMaxTokens
	}
	upstreamModel := s.MapModel(&provider, model)
	requestPath, requestBody, err := providerDebugRequest(provider.Type, upstreamModel, prompt, maxTokens)
	if err != nil {
		return nil, err
	}

	result := &ProviderChatDebugResult{
		ProviderType:  provider.Type,
		Model:         model,
		UpstreamModel: upstreamModel,
		RequestPath:   requestPath,
	}
	requestCtx, cancel := context.WithTimeout(ctx, providerDebugTimeout)
	defer cancel()
	headers := make(http.Header)
	headers.Set("Accept", "application/json")
	headers.Set("Content-Type", "application/json")
	start := time.Now()
	relayResult, forwardErr := RelayServiceApp.forward(requestCtx, &provider, http.MethodPost, requestPath, requestBody, headers, "")
	result.ResponseTimeMs = time.Since(start).Milliseconds()
	if forwardErr != nil {
		result.Message = forwardErr.Error()
		return result, nil
	}
	if relayResult == nil {
		result.Message = "upstream response is empty"
		return result, nil
	}
	result.StatusCode = relayResult.StatusCode
	result.FirstResponseTimeMs = relayResult.FirstResponseTimeMs
	result.Usage = providerDebugUsage(relayResult.Body, relayResult.Usage)
	result.Response = providerDebugResponse(relayResult.Body)
	result.Content = providerDebugContent(result.Response)
	if relayResult.StatusCode < http.StatusOK || relayResult.StatusCode >= http.StatusMultipleChoices {
		result.Message = providerDebugErrorMessage(result.Response)
		if result.Message == "" {
			result.Message = http.StatusText(relayResult.StatusCode)
		}
		return result, nil
	}
	if result.Content == "" {
		result.Message = "上游返回成功，但未解析到对话内容"
		return result, nil
	}
	result.OK = true
	result.Message = "模型对话验证成功"
	return result, nil
}

func (s *ProviderService) hydrateProviderDebugSecrets(provider *domains.VendorMeta) error {
	if provider == nil || strings.TrimSpace(provider.Guid) == "" {
		return nil
	}
	existing, err := s.GetByGUID(provider.Guid)
	if err != nil {
		return err
	}
	if strings.TrimSpace(provider.Key) == "" {
		provider.Key = existing.Key
	}
	if strings.TrimSpace(provider.ProxyPassword) == "" {
		provider.ProxyPassword = existing.ProxyPassword
	}
	return nil
}

func providerDebugRequest(providerType string, model string, prompt string, maxTokens int) (string, []byte, error) {
	var path string
	var payload any
	switch providerType {
	case constants.ProviderTypeAnthropic:
		path = "/v1/messages"
		payload = map[string]any{
			"model":      model,
			"max_tokens": maxTokens,
			"messages":   []map[string]string{{"role": "user", "content": prompt}},
		}
	case constants.ProviderTypeGemini:
		path = "/v1beta/models/" + url.PathEscape(model) + ":generateContent"
		payload = map[string]any{
			"contents": []map[string]any{{"parts": []map[string]string{{"text": prompt}}}},
			"generationConfig": map[string]any{
				"maxOutputTokens": maxTokens,
			},
		}
	default:
		path = "/v1/chat/completions"
		request := map[string]any{
			"model":    model,
			"messages": []map[string]string{{"role": "user", "content": prompt}},
			"stream":   false,
		}
		request[providerDebugTokenLimitField(model)] = maxTokens
		payload = request
	}
	body, err := json.Marshal(payload)
	return path, body, err
}

func providerDebugTokenLimitField(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	for _, prefix := range []string{"gpt-5", "o1", "o3", "o4"} {
		if strings.HasPrefix(model, prefix) {
			return "max_completion_tokens"
		}
	}
	return "max_tokens"
}

func providerDebugUsage(body []byte, fallback vos.Usage) vos.Usage {
	if hasUsageTokens(fallback) {
		return normalizeUsage(fallback)
	}
	var payload struct {
		UsageMetadata struct {
			PromptTokenCount     int64 `json:"promptTokenCount"`
			CandidatesTokenCount int64 `json:"candidatesTokenCount"`
			TotalTokenCount      int64 `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return fallback
	}
	usage := vos.Usage{
		PromptTokens:     payload.UsageMetadata.PromptTokenCount,
		CompletionTokens: payload.UsageMetadata.CandidatesTokenCount,
		TotalTokens:      payload.UsageMetadata.TotalTokenCount,
	}
	return normalizeUsage(usage)
}

func providerDebugResponse(body []byte) any {
	var value any
	if err := json.Unmarshal(body, &value); err == nil {
		return value
	}
	text := strings.TrimSpace(string(body))
	if len(text) > 4000 {
		text = text[:4000] + "..."
	}
	return map[string]any{"text": text}
}

func providerDebugContent(value any) string {
	root, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	if content := providerDebugOpenAIContent(root); content != "" {
		return content
	}
	if content := providerDebugAnthropicContent(root); content != "" {
		return content
	}
	if content := providerDebugGeminiContent(root); content != "" {
		return content
	}
	return strings.TrimSpace(valueString(root["output_text"]))
}

func providerDebugOpenAIContent(root map[string]any) string {
	choices, _ := root["choices"].([]any)
	if len(choices) == 0 {
		return ""
	}
	choice, _ := choices[0].(map[string]any)
	message, _ := choice["message"].(map[string]any)
	if content := valueString(message["content"]); content != "" {
		return strings.TrimSpace(content)
	}
	if parts, ok := message["content"].([]any); ok {
		if content := providerDebugTextParts(parts, "text"); content != "" {
			return content
		}
	}
	return strings.TrimSpace(valueString(choice["text"]))
}

func providerDebugAnthropicContent(root map[string]any) string {
	content, _ := root["content"].([]any)
	return providerDebugTextParts(content, "text")
}

func providerDebugGeminiContent(root map[string]any) string {
	candidates, _ := root["candidates"].([]any)
	if len(candidates) == 0 {
		return ""
	}
	candidate, _ := candidates[0].(map[string]any)
	content, _ := candidate["content"].(map[string]any)
	parts, _ := content["parts"].([]any)
	return providerDebugTextParts(parts, "text")
}

func providerDebugTextParts(parts []any, key string) string {
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		row, _ := part.(map[string]any)
		if text := strings.TrimSpace(valueString(row[key])); text != "" {
			texts = append(texts, text)
		}
	}
	return strings.Join(texts, "\n")
}

func providerDebugErrorMessage(value any) string {
	root, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	if errorValue, ok := root["error"].(map[string]any); ok {
		if message := strings.TrimSpace(valueString(errorValue["message"])); message != "" {
			return message
		}
	}
	for _, key := range []string{"message", "msg", "detail", "text"} {
		if message := strings.TrimSpace(valueString(root[key])); message != "" {
			return message
		}
	}
	return ""
}

func valueString(value any) string {
	text, _ := value.(string)
	return text
}
