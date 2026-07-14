package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"navapi-go/constants"
	"navapi-go/domains"
)

func TestProviderDebugRequestUsesProviderProtocol(t *testing.T) {
	tests := []struct {
		providerType string
		wantPath     string
	}{
		{providerType: constants.ProviderTypeOpenAI, wantPath: "/v1/chat/completions"},
		{providerType: constants.ProviderTypeAnthropic, wantPath: "/v1/messages"},
		{providerType: constants.ProviderTypeGemini, wantPath: "/v1beta/models/gemini-2.5-pro:generateContent"},
	}
	for _, test := range tests {
		path, body, err := providerDebugRequest(test.providerType, "gemini-2.5-pro", "ping", 64)
		if err != nil {
			t.Fatalf("build %s request: %v", test.providerType, err)
		}
		if path != test.wantPath {
			t.Fatalf("expected %s path %q, got %q", test.providerType, test.wantPath, path)
		}
		if !json.Valid(body) {
			t.Fatalf("expected %s request body to be valid JSON", test.providerType)
		}
	}
}

func TestProviderDebugRequestUsesCompletionLimitForReasoningModels(t *testing.T) {
	_, body, err := providerDebugRequest(constants.ProviderTypeOpenAI, "gpt-5.5", "ping", 64)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if payload["max_completion_tokens"] != float64(64) {
		t.Fatalf("expected max_completion_tokens=64, got %v", payload["max_completion_tokens"])
	}
	if _, exists := payload["max_tokens"]; exists {
		t.Fatal("did not expect max_tokens for gpt-5.5")
	}
}

func TestProviderDebugChatCallsUpstream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected request path %q", r.URL.Path)
		}
		if authorization := r.Header.Get("Authorization"); authorization != "Bearer debug-key" {
			t.Errorf("unexpected authorization %q", authorization)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		if model := body["model"]; model != "upstream-model" {
			t.Errorf("unexpected upstream model %v", model)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"pong"}}],"usage":{"prompt_tokens":3,"completion_tokens":1,"total_tokens":4}}`))
	}))
	defer server.Close()

	service := &ProviderService{}
	result, err := service.DebugChat(context.Background(), ProviderChatDebugInput{
		Provider: domains.VendorMeta{
			Type:          constants.ProviderTypeOpenAI,
			BaseURL:       server.URL,
			ModelOverride: "upstream-model",
		},
		Key:       "debug-key",
		Model:     "public-model",
		Prompt:    "ping",
		MaxTokens: 16,
	})
	if err != nil {
		t.Fatalf("debug chat: %v", err)
	}
	if !result.OK || result.Content != "pong" {
		t.Fatalf("expected successful pong response, got %+v", result)
	}
	if result.Usage.TotalTokens != 4 {
		t.Fatalf("expected 4 total tokens, got %d", result.Usage.TotalTokens)
	}
}

func TestProviderDebugContentParsesSupportedResponses(t *testing.T) {
	tests := []struct {
		name  string
		value any
	}{
		{
			name: "openai",
			value: map[string]any{
				"choices": []any{map[string]any{"message": map[string]any{"content": "pong"}}},
			},
		},
		{
			name:  "anthropic",
			value: map[string]any{"content": []any{map[string]any{"type": "text", "text": "pong"}}},
		},
		{
			name: "gemini",
			value: map[string]any{
				"candidates": []any{map[string]any{
					"content": map[string]any{"parts": []any{map[string]any{"text": "pong"}}},
				}},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if content := providerDebugContent(test.value); content != "pong" {
				t.Fatalf("expected pong, got %q", content)
			}
		})
	}
}
