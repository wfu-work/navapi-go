package services

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"navapi-go/domains"
)

func TestTestBalanceDetectsSub2FromGenericHTML(t *testing.T) {
	var genericRequests int
	var sub2Requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected authorization header: %q", r.Header.Get("Authorization"))
		}
		switch r.URL.Path {
		case "/user/balance":
			genericRequests++
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte("<!doctype html><html><body>Sub2</body></html>"))
		case "/v1/usage":
			sub2Requests++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"isValid":true,"planName":"Pro","remaining":7.5,"quota":{"limit":10,"used":2.5}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider := &domains.VendorMeta{
		BaseURL:              server.URL + "/v1",
		Key:                  "test-key",
		BalanceTemplate:      balanceTemplateGeneric,
		BalanceCustomPath:    "/user/balance",
		BalanceRemainingPath: "remaining",
		BalanceMultiplier:    1,
		BalanceUnit:          "USD",
	}
	result, err := (&ProviderService{}).TestBalance(provider)
	if err != nil {
		t.Fatalf("test balance: %v", err)
	}
	if !result.OK || result.Template != balanceTemplateSub2 || result.DetectedTemplate != balanceTemplateSub2 {
		t.Fatalf("unexpected detection result: %+v", result)
	}
	if result.TargetURL != server.URL+"/v1/usage" {
		t.Fatalf("unexpected target URL: %s", result.TargetURL)
	}
	assertBalanceNumber(t, "remaining", result.Remaining, 7.5)
	if genericRequests != 1 || sub2Requests != 1 {
		t.Fatalf("unexpected request counts: generic=%d sub2=%d", genericRequests, sub2Requests)
	}
}

func TestShouldProbeSub2OnlyForDefaultGenericHTMLResponse(t *testing.T) {
	htmlResult := &ProviderBalanceResult{
		StatusCode:  http.StatusOK,
		ContentType: "text/html; charset=utf-8",
	}
	tests := []struct {
		name     string
		provider *domains.VendorMeta
		result   *ProviderBalanceResult
		want     bool
	}{
		{
			name: "default generic html",
			provider: &domains.VendorMeta{
				BalanceTemplate:   balanceTemplateGeneric,
				BalanceCustomPath: "/user/balance",
			},
			result: htmlResult,
			want:   true,
		},
		{
			name: "custom generic path",
			provider: &domains.VendorMeta{
				BalanceTemplate:   balanceTemplateGeneric,
				BalanceCustomPath: "/custom/balance",
			},
			result: htmlResult,
		},
		{
			name: "generic json failure",
			provider: &domains.VendorMeta{
				BalanceTemplate:   balanceTemplateGeneric,
				BalanceCustomPath: "/user/balance",
			},
			result: &ProviderBalanceResult{
				StatusCode:  http.StatusOK,
				ContentType: "application/json",
			},
		},
		{
			name: "html body with incorrect content type",
			provider: &domains.VendorMeta{
				BalanceTemplate:   balanceTemplateGeneric,
				BalanceCustomPath: "/user/balance",
			},
			result: &ProviderBalanceResult{
				StatusCode:   http.StatusOK,
				ContentType:  "text/plain",
				htmlResponse: true,
			},
			want: true,
		},
		{
			name: "explicit sub2 template",
			provider: &domains.VendorMeta{
				BalanceTemplate:   balanceTemplateSub2,
				BalanceCustomPath: "/v1/usage",
			},
			result: htmlResult,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldProbeSub2Balance(tt.provider, tt.result); got != tt.want {
				t.Fatalf("shouldProbeSub2Balance() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplySub2BalanceDefaultsOverwritesGenericPaths(t *testing.T) {
	provider := &domains.VendorMeta{
		BalanceTemplate:      balanceTemplateGeneric,
		BalanceCustomPath:    "/user/balance",
		BalanceRemainingPath: "data.balance",
		BalanceMultiplier:    0.5,
		BalanceUnit:          "CNY",
		BalanceTotalPath:     "total",
		BalanceUsedPath:      "used",
		BalancePlanPath:      "plan",
		BalanceValidPath:     "active",
		BalanceErrorPath:     "error",
	}
	applySub2BalanceDefaults(provider)

	if provider.BalanceTemplate != balanceTemplateSub2 || provider.BalanceCustomPath != "/v1/usage" {
		t.Fatalf("unexpected template defaults: %+v", provider)
	}
	if provider.BalanceRemainingPath != "remaining" || provider.BalanceTotalPath != "quota.limit" || provider.BalanceUsedPath != "quota.used" {
		t.Fatalf("unexpected amount paths: %+v", provider)
	}
	if provider.BalancePlanPath != "planName" || provider.BalanceValidPath != "isValid" || provider.BalanceMultiplier != 1 || provider.BalanceUnit != "USD" {
		t.Fatalf("unexpected metadata defaults: %+v", provider)
	}
}

func TestSub2BalanceProbeURLKeepsOnlyOrigin(t *testing.T) {
	targetURL, err := sub2BalanceProbeURL("https://user:pass@sub2.example/user/balance?from=test#result")
	if err != nil {
		t.Fatalf("build sub2 probe URL: %v", err)
	}
	if targetURL != "https://sub2.example/v1/usage" {
		t.Fatalf("unexpected probe URL: %s", targetURL)
	}
}

func TestQueryBalanceDoesNotExposeHTMLBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!doctype html><html><body>sensitive page content</body></html>"))
	}))
	defer server.Close()

	provider := &domains.VendorMeta{
		BalanceTemplate:      balanceTemplateGeneric,
		BalanceCustomPath:    server.URL,
		BalanceRemainingPath: "remaining",
		BalanceMultiplier:    1,
		BalanceUnit:          "USD",
	}
	result, err := (&ProviderService{}).queryBalance(provider)
	if err != nil {
		t.Fatalf("query balance: %v", err)
	}
	if result.OK || result.Message != "余额接口返回了网页内容；该服务可能是 Sub2API，请使用 Sub2API 模板（/v1/usage）" {
		t.Fatalf("unexpected HTML response result: %+v", result)
	}
}

func TestSub2BalanceTemplate(t *testing.T) {
	provider := &domains.VendorMeta{
		BaseURL:         "https://sub2.example/v1",
		BalanceTemplate: balanceTemplateSub2,
	}
	normalizeProviderBalanceConfig(provider)

	if provider.BalanceCustomPath != "/v1/usage" {
		t.Fatalf("unexpected balance path: %s", provider.BalanceCustomPath)
	}
	targetURL, err := providerBalanceTargetURL(provider)
	if err != nil {
		t.Fatalf("build target URL: %v", err)
	}
	if targetURL != "https://sub2.example/v1/usage" {
		t.Fatalf("unexpected target URL: %s", targetURL)
	}

	result := &ProviderBalanceResult{}
	applyBalancePayload(result, provider, map[string]any{
		"mode":      "quota_limited",
		"isValid":   true,
		"planName":  "Pro",
		"remaining": 7.5,
		"quota": map[string]any{
			"limit": 10.0,
			"used":  2.5,
		},
	})

	assertBalanceNumber(t, "remaining", result.Remaining, 7.5)
	assertBalanceNumber(t, "total", result.Total, 10)
	assertBalanceNumber(t, "used", result.Used, 2.5)
	if result.Plan != "Pro" {
		t.Fatalf("unexpected plan: %s", result.Plan)
	}
	if result.Valid == nil || !*result.Valid {
		t.Fatalf("unexpected valid state: %v", result.Valid)
	}
}

func TestSub2BalanceTemplateWalletMode(t *testing.T) {
	provider := &domains.VendorMeta{BalanceTemplate: balanceTemplateSub2, BalanceMultiplier: 1}
	result := &ProviderBalanceResult{}
	applyBalancePayload(result, provider, map[string]any{
		"mode":      "unrestricted",
		"isValid":   true,
		"planName":  "钱包余额",
		"remaining": 24.8,
		"balance":   24.8,
	})

	assertBalanceNumber(t, "remaining", result.Remaining, 24.8)
	if result.Plan != "钱包余额" {
		t.Fatalf("unexpected plan: %s", result.Plan)
	}
}

func assertBalanceNumber(t *testing.T, name string, value *float64, expected float64) {
	t.Helper()
	if value == nil || *value != expected {
		t.Fatalf("unexpected %s: %v", name, value)
	}
}
