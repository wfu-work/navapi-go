package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"navapi-go/domains"
)

func TestProviderRequestDebugSendsCustomRequest(t *testing.T) {
	var receivedBody map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer saved-provider-key" {
			t.Fatalf("authorization = %q", got)
		}
		if got := r.Header.Get("X-Debug-Trace"); got != "enabled" {
			t.Fatalf("X-Debug-Trace = %q", got)
		}
		if got := r.URL.Query().Get("api-version"); got != "2026-01-01" {
			t.Fatalf("api-version = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"debug-response","ok":true}`))
	}))
	defer upstream.Close()

	result, err := executeProviderRequestDebug(context.Background(), &domains.VendorMeta{Key: "saved-provider-key"}, ProviderRequestDebugInput{
		Method:   http.MethodPost,
		URL:      upstream.URL + "/v1/chat/completions",
		AuthType: "bearer",
		Query:    map[string]string{"api-version": "2026-01-01"},
		Headers:  map[string]string{"X-Debug-Trace": "enabled"},
		Body:     `{"model":"gpt-test"}`,
	})
	if err != nil {
		t.Fatalf("executeProviderRequestDebug() error = %v", err)
	}
	if !result.OK || result.StatusCode != http.StatusCreated {
		t.Fatalf("result = %+v", result)
	}
	if receivedBody["model"] != "gpt-test" {
		t.Fatalf("body = %#v", receivedBody)
	}
	response, ok := result.Response.(map[string]any)
	if !ok || response["id"] != "debug-response" {
		t.Fatalf("response = %#v", result.Response)
	}
}

func TestProviderRequestDebugSupportsQueryTokenAndMasksResultURL(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("key"); got != "temporary-key" {
			t.Fatalf("key = %q", got)
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("pong"))
	}))
	defer upstream.Close()

	result, err := executeProviderRequestDebug(context.Background(), &domains.VendorMeta{Key: "saved-key"}, ProviderRequestDebugInput{
		Method:   http.MethodGet,
		URL:      upstream.URL + "/models",
		AuthType: "query",
		AuthName: "key",
		Token:    "temporary-key",
	})
	if err != nil {
		t.Fatalf("executeProviderRequestDebug() error = %v", err)
	}
	if strings.Contains(result.TargetURL, "temporary-key") || !strings.Contains(result.TargetURL, "%2A%2A%2A%2A%2A%2A") {
		t.Fatalf("target url was not masked: %s", result.TargetURL)
	}
	response, ok := result.Response.(map[string]any)
	if !ok || response["text"] != "pong" {
		t.Fatalf("response = %#v", result.Response)
	}
}

func TestProviderRequestDebugWorksWithoutProvider(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer manual-token" {
			t.Fatalf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"standalone-model"}]}`))
	}))
	defer upstream.Close()

	result, err := new(ProviderService).DebugRequest(context.Background(), ProviderRequestDebugInput{
		Method:   http.MethodGet,
		URL:      upstream.URL + "/v1/models",
		AuthType: "bearer",
		Token:    "manual-token",
	})
	if err != nil {
		t.Fatalf("DebugRequest() error = %v", err)
	}
	if !result.OK || result.StatusCode != http.StatusOK {
		t.Fatalf("result = %+v", result)
	}
}

func TestProviderRequestDebugRejectsUnsafeRequestOptions(t *testing.T) {
	tests := []struct {
		name  string
		input ProviderRequestDebugInput
	}{
		{name: "scheme", input: ProviderRequestDebugInput{Method: http.MethodGet, URL: "file:///tmp/key", AuthType: "none"}},
		{name: "method", input: ProviderRequestDebugInput{Method: http.MethodConnect, URL: "https://example.com", AuthType: "none"}},
		{name: "header", input: ProviderRequestDebugInput{Method: http.MethodGet, URL: "https://example.com", AuthType: "none", Headers: map[string]string{"Host": "other.example.com"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := executeProviderRequestDebug(context.Background(), &domains.VendorMeta{}, tt.input); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
