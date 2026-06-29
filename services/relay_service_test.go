package services

import (
	"encoding/json"
	"net/url"
	"testing"

	"navapi-go/dto"
)

func TestParseUsageNormalizesOpenAIAndResponsesShapes(t *testing.T) {
	body := []byte(`{"usage":{"input_tokens":7,"output_tokens":11,"input_tokens_details":{"cached_tokens":3}}}`)

	usage := parseUsage(body, "application/json")

	if usage.PromptTokens != 7 {
		t.Fatalf("prompt tokens = %d, want 7", usage.PromptTokens)
	}
	if usage.CompletionTokens != 11 {
		t.Fatalf("completion tokens = %d, want 11", usage.CompletionTokens)
	}
	if usage.CachedTokens != 3 {
		t.Fatalf("cached tokens = %d, want 3", usage.CachedTokens)
	}
	if usage.TotalTokens != 18 {
		t.Fatalf("total tokens = %d, want 18", usage.TotalTokens)
	}
}

func TestStreamUsageTrackerParsesSplitSSEChunks(t *testing.T) {
	tracker := &streamUsageTracker{}

	tracker.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"he\"}}]}\n"))
	tracker.Write([]byte("data: {\"usage\":{\"prompt_tokens\":4,"))
	tracker.Write([]byte("\"completion_tokens\":6,\"total_tokens\":10}}\n"))
	tracker.Write([]byte("data: [DONE]\n"))

	usage := tracker.Finish()
	if usage != (dto.Usage{PromptTokens: 4, CompletionTokens: 6, TotalTokens: 10}) {
		t.Fatalf("usage = %+v, want prompt=4 completion=6 total=10", usage)
	}
}

func TestRewriteBodyModelOnlyTouchesJSONModel(t *testing.T) {
	body := []byte(`{"model":"public-model","messages":[{"role":"user","content":"hi"}]}`)

	rewritten := rewriteBodyModel(body, "upstream-model", "application/json")
	var payload map[string]any
	if err := json.Unmarshal(rewritten, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["model"] != "upstream-model" {
		t.Fatalf("model = %v, want upstream-model", payload["model"])
	}

	unchanged := rewriteBodyModel(body, "ignored", "multipart/form-data")
	if string(unchanged) != string(body) {
		t.Fatalf("multipart body changed: %s", unchanged)
	}
}

func TestAttachGeminiKeyPreservesIncomingQuery(t *testing.T) {
	target := attachGeminiKey("https://example.test/v1beta/models/gemini:generateContent", "secret", "alt=sse")
	parsed, err := url.Parse(target)
	if err != nil {
		t.Fatal(err)
	}

	query := parsed.Query()
	if query.Get("key") != "secret" {
		t.Fatalf("key = %q, want secret", query.Get("key"))
	}
	if query.Get("alt") != "sse" {
		t.Fatalf("alt = %q, want sse", query.Get("alt"))
	}
}

func TestCalculateFinalQuotaKeepsReservedQuotaWhenUsageMissing(t *testing.T) {
	quota := calculateFinalQuota("gpt-test", "default", dto.Usage{}, []byte(`{"model":"gpt-test"}`), 42)

	if quota != 42 {
		t.Fatalf("quota = %d, want reserved quota 42", quota)
	}
}
