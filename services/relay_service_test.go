package services

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"navapi-go/constants"
	"navapi-go/domains"
	"navapi-go/vos"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/global"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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

func TestParseUsageReadsNestedResponsesUsage(t *testing.T) {
	body := []byte(`{"type":"response.completed","response":{"usage":{"input_tokens":26,"output_tokens":5,"total_tokens":31}}}`)

	usage := parseUsage(body, "application/json")

	if usage != (vos.Usage{PromptTokens: 26, CompletionTokens: 5, TotalTokens: 31, InputTokens: 26, OutputTokens: 5}) {
		t.Fatalf("usage = %+v, want nested responses usage", usage)
	}
}

func TestStreamUsageTrackerParsesSplitSSEChunks(t *testing.T) {
	tracker := &streamUsageTracker{}

	tracker.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"he\"}}]}\n"))
	tracker.Write([]byte("data: {\"usage\":{\"prompt_tokens\":4,"))
	tracker.Write([]byte("\"completion_tokens\":6,\"total_tokens\":10}}\n"))
	tracker.Write([]byte("data: [DONE]\n"))

	usage := tracker.Finish()
	if usage != (vos.Usage{PromptTokens: 4, CompletionTokens: 6, TotalTokens: 10}) {
		t.Fatalf("usage = %+v, want prompt=4 completion=6 total=10", usage)
	}
}

func TestStreamUsageTrackerParsesNestedResponsesUsage(t *testing.T) {
	tracker := &streamUsageTracker{}

	tracker.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"pong\"}\n\n"))
	tracker.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":26,"))
	tracker.Write([]byte("\"output_tokens\":5,\"total_tokens\":31}}}\n\n"))
	tracker.Write([]byte("data: [DONE]\n\n"))

	usage := tracker.Finish()
	if usage != (vos.Usage{PromptTokens: 26, CompletionTokens: 5, TotalTokens: 31, InputTokens: 26, OutputTokens: 5}) {
		t.Fatalf("usage = %+v, want nested responses stream usage", usage)
	}
}

func TestEnsureOpenAIStreamUsageAddsIncludeUsage(t *testing.T) {
	body := []byte(`{"model":"gpt-test","stream":true,"messages":[{"role":"user","content":"hi"}]}`)

	next := ensureOpenAIStreamUsage(body, "application/json")
	var payload map[string]any
	if err := json.Unmarshal(next, &payload); err != nil {
		t.Fatal(err)
	}
	options, ok := payload["stream_options"].(map[string]any)
	if !ok {
		t.Fatalf("stream_options = %#v, want object", payload["stream_options"])
	}
	if options["include_usage"] != true {
		t.Fatalf("include_usage = %#v, want true", options["include_usage"])
	}
}

func TestEnsureOpenAIStreamUsageSkipsNonStreamRequest(t *testing.T) {
	body := []byte(`{"model":"gpt-test","stream":false}`)

	next := ensureOpenAIStreamUsage(body, "application/json")
	if string(next) != string(body) {
		t.Fatalf("non-stream body changed: %s", string(next))
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

	rewrittenWithoutContentType := rewriteBodyModel(body, "upstream-model", "")
	if err := json.Unmarshal(rewrittenWithoutContentType, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["model"] != "upstream-model" {
		t.Fatalf("model without content type = %v, want upstream-model", payload["model"])
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
	quota := calculateFinalQuota("gpt-test", "default", vos.Usage{}, []byte(`{"model":"gpt-test"}`), 42)

	if quota != 42 {
		t.Fatalf("quota = %d, want reserved quota 42", quota)
	}
}

func TestRelayHTTPForwardsOpenAIChatAndSettlesQuota(t *testing.T) {
	db := withRelayTestDB(t)

	var upstreamBody map[string]any
	upstreamRequests := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamRequests++
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %s, want /v1/chat/completions", r.URL.Path)
		}
		if r.URL.Query().Get("timeout") != "30" {
			t.Errorf("timeout query = %q, want 30", r.URL.Query().Get("timeout"))
		}
		if r.Header.Get("Authorization") != "Bearer sk-provider" {
			t.Errorf("authorization = %q, want provider bearer", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Errorf("decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", "upstream-req-1")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-test","object":"chat.completion","usage":{"prompt_tokens":8,"completion_tokens":4,"total_tokens":12,"prompt_tokens_details":{"cached_tokens":2}},"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer upstream.Close()

	provider := domains.VendorMeta{
		VendorName:   "openai-compatible",
		DisplayName:  "OpenAI Compatible",
		Type:         constants.ProviderTypeOpenAI,
		BaseURL:      upstream.URL,
		Key:          "sk-provider",
		Models:       "public-model",
		ModelMapping: `{"public-model":"upstream-model"}`,
		Enabled:      true,
	}
	if err := db.Create(&provider).Error; err != nil {
		t.Fatal(err)
	}
	token := domains.ApiToken{
		UserGuid:       "user-1",
		Name:           "Client Token",
		Key:            "sk-client",
		Status:         constants.StatusEnabled,
		Group:          constants.DefaultGroup,
		RemainQuota:    1000,
		UnlimitedQuota: false,
	}
	if err := db.Create(&token).Error; err != nil {
		t.Fatal(err)
	}
	account := domains.UserQuota{
		UserGuid:    "user-1",
		RemainQuota: 1000,
		TotalQuota:  1000,
		Group:       constants.DefaultGroup,
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions?timeout=30", strings.NewReader(`{"model":"public-model","reasoning_effort":"medium","messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer sk-client")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-Id", "client-req-1")
	c.Request = req

	result, streamed, err := RelayServiceApp.RelayHTTP(c, &token, RelayEndpoint{
		UpstreamPath: "/v1/chat/completions",
		Method:       http.MethodPost,
		Format:       constants.ProviderTypeOpenAI,
	})
	if err != nil {
		t.Fatal(err)
	}
	if streamed {
		t.Fatal("streamed = true, want buffered result")
	}
	if result.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", result.StatusCode, string(result.Body))
	}
	if upstreamRequests != 1 {
		t.Fatalf("upstream requests = %d, want 1", upstreamRequests)
	}
	if upstreamBody["model"] != "upstream-model" {
		t.Fatalf("upstream model = %v, want upstream-model", upstreamBody["model"])
	}
	if result.Usage != (vos.Usage{PromptTokens: 8, CompletionTokens: 4, TotalTokens: 12, CachedTokens: 2, PromptTokensDetails: vos.TokenUsageDetails{CachedTokens: 2}}) {
		t.Fatalf("usage = %+v, want 8/4/12", result.Usage)
	}

	var storedToken domains.ApiToken
	if err := db.First(&storedToken, token.Id).Error; err != nil {
		t.Fatal(err)
	}
	if storedToken.UsedQuota != 12 || storedToken.RemainQuota != 988 {
		t.Fatalf("token quota used=%d remain=%d, want 12/988", storedToken.UsedQuota, storedToken.RemainQuota)
	}
	var storedAccount domains.UserQuota
	if err := db.Where("user_guid = ?", "user-1").First(&storedAccount).Error; err != nil {
		t.Fatal(err)
	}
	if storedAccount.UsedQuota != 12 {
		t.Fatalf("user used quota = %d, want 12", storedAccount.UsedQuota)
	}
	var log domains.UsageLog
	if err := db.First(&log).Error; err != nil {
		t.Fatal(err)
	}
	if log.Status != "success" || log.Quota != 12 || log.PromptTokens != 8 || log.CompletionTokens != 4 {
		t.Fatalf("usage log = %+v, want successful 12 quota log", log)
	}
	if log.ProviderGuid != provider.Guid || log.TokenGuid != token.Guid || log.UpstreamRequestID != "upstream-req-1" || log.RequestID != "client-req-1" {
		t.Fatalf("usage log ids = provider:%q token:%q upstream:%q request:%q", log.ProviderGuid, log.TokenGuid, log.UpstreamRequestID, log.RequestID)
	}
	var other map[string]any
	if err := json.Unmarshal([]byte(log.Other), &other); err != nil {
		t.Fatalf("usage log other json: %v", err)
	}
	if other["reasoningEffort"] != "medium" || other["cachedTokens"] != float64(2) || other["group"] != constants.DefaultGroup {
		t.Fatalf("usage log other = %+v, want reasoning/cached/group metadata", other)
	}
	var wallet domains.UserWallet
	if err := db.Where("user_guid = ?", "user-1").First(&wallet).Error; err != nil {
		t.Fatal(err)
	}
	if wallet.BalanceQuota != 988 || wallet.PaidBalanceQuota != 988 || wallet.TotalConsumedQuota != 12 || wallet.TotalRequestCount != 1 || wallet.TotalRechargeQuota != 1000 {
		t.Fatalf("wallet = %+v, want balance=988 consumed=12 requests=1", wallet)
	}
	var walletRecord domains.UserWalletRecord
	if err := db.Where("user_guid = ? AND type = ?", "user-1", domains.WalletRecordTypeConsume).First(&walletRecord).Error; err != nil {
		t.Fatal(err)
	}
	if walletRecord.QuotaDelta != -12 || walletRecord.BalanceAfter != 988 || walletRecord.TokenGuid != token.Guid {
		t.Fatalf("wallet record = %+v, want consume delta -12 balance 988", walletRecord)
	}
}

func TestRelayHTTPRejectsWhenUserConcurrencyLimitReached(t *testing.T) {
	withRelayTestDB(t)

	if _, err := UserSettingsServiceApp.Save("user-1", &domains.UserSettings{
		QuotaReminderEnabled:        true,
		PlatformAnnouncementEnabled: true,
		MaxConcurrency:              1,
		ExtraConfig:                 "{}",
	}); err != nil {
		t.Fatal(err)
	}
	release, err := UserConcurrencyServiceApp.Acquire("user-1")
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	c := newRelayTestContext(`{"model":"public-model","messages":[{"role":"user","content":"again"}]}`)
	token := domains.ApiToken{UserGuid: "user-1"}
	result, streamed, err := RelayServiceApp.RelayHTTP(c, &token, RelayEndpoint{
		UpstreamPath: "/v1/chat/completions",
		Method:       http.MethodPost,
		Format:       constants.ProviderTypeOpenAI,
	})
	if err == nil {
		t.Fatal("relay succeeded, want concurrency limit error")
	}
	var relayErr *RelayHTTPError
	if !errors.As(err, &relayErr) || relayErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("relay error = %v, want 429 RelayHTTPError", err)
	}
	if result != nil || streamed {
		t.Fatalf("relay result=%+v streamed=%v, want no result", result, streamed)
	}
}

func withRelayTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := global.NAV_DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&domains.ApiToken{},
		&domains.UserQuota{},
		&domains.UserWallet{},
		&domains.UserWalletRecord{},
		&domains.UserSettings{},
		&domains.VendorMeta{},
		&domains.UsageLog{},
		&domains.Option{},
		&domains.ModelMeta{},
		&domains.ModelGroup{},
		&domains.Pricing{},
	); err != nil {
		t.Fatal(err)
	}
	global.NAV_DB = db
	UserConcurrencyServiceApp.reset()
	OptionServiceApp.mu.Lock()
	OptionServiceApp.cache = map[string]string{}
	OptionServiceApp.mu.Unlock()
	if err := db.Create(&domains.ModelGroup{
		GroupName:       constants.DefaultGroup,
		DisplayName:     "Default",
		QuotaMultiplier: 1,
		Enabled:         true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		global.NAV_DB = previousDB
		UserConcurrencyServiceApp.reset()
		OptionServiceApp.mu.Lock()
		OptionServiceApp.cache = map[string]string{}
		OptionServiceApp.mu.Unlock()
	})
	return db
}

func newRelayTestContext(body string) *gin.Context {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer sk-client")
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	return c
}
