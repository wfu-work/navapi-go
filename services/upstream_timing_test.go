package services

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"navapi-go/constants"
	"navapi-go/domains"

	"github.com/gin-gonic/gin"
)

func TestForwardStreamCapturesUpstreamTiming(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		time.Sleep(15 * time.Millisecond)
		_, _ = io.WriteString(w, "data: {\"choices\":[]}\n\ndata: [DONE]\n\n")
		w.(http.Flusher).Flush()
	}))
	defer server.Close()

	body := []byte(`{"model":"gpt-test","stream":true}`)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "http://gateway.test/v1/responses", bytes.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	provider := domains.VendorMeta{
		VendorName: "timing-test",
		Type:       constants.ProviderTypeOpenAI,
		BaseURL:    server.URL,
		Key:        "test-key",
		Enabled:    true,
	}

	result, err := (RelayService{client: server.Client()}).forwardStream(
		context,
		&provider,
		http.MethodPost,
		"/v1/responses",
		body,
		context.Request.Header,
		"",
		false,
	)
	if err != nil {
		t.Fatalf("forward stream: %v", err)
	}
	if result == nil {
		t.Fatal("expected relay result")
	}
	if result.Timing.RequestBodyBytes != int64(len(body)) {
		t.Fatalf("expected request size %d, got %d", len(body), result.Timing.RequestBodyBytes)
	}
	if result.FirstResponseTimeMs < 10 {
		t.Fatalf("expected delayed first response, got %dms", result.FirstResponseTimeMs)
	}
	if result.Timing.ResponseHeaderTimeMs > result.FirstResponseTimeMs {
		t.Fatalf("response header %dms exceeds first body %dms", result.Timing.ResponseHeaderTimeMs, result.FirstResponseTimeMs)
	}
	if result.Timing.UpstreamTotalTimeMs < result.FirstResponseTimeMs {
		t.Fatalf("upstream total %dms is less than first body %dms", result.Timing.UpstreamTotalTimeMs, result.FirstResponseTimeMs)
	}
	if recorder.Body.Len() == 0 {
		t.Fatal("expected streamed response body")
	}
	if recorder.Header().Get("X-Accel-Buffering") != "no" {
		t.Fatalf("expected streaming proxy buffering to be disabled, got %q", recorder.Header().Get("X-Accel-Buffering"))
	}
}
