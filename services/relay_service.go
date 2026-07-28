package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptrace"
	"net/textproto"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"navapi-go/constants"
	"navapi-go/domains"
	"navapi-go/vos"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RelayService struct {
	client                  *http.Client
	streamClient            *http.Client
	streamHeartbeatInterval time.Duration
}

var RelayServiceApp = new(RelayService{
	client:       &http.Client{Timeout: 10 * time.Minute, Transport: cloneDefaultTransport()},
	streamClient: newStreamHTTPClient(),
})

const (
	defaultStreamHeartbeatInterval = 15 * time.Second
	streamHeartbeatComment         = ": navapi keep-alive\n\n"
)

type RelayEndpoint struct {
	UpstreamPath  string
	Method        string
	Format        string
	ModelFromPath bool
	NoBilling     bool
	DefaultModel  string
}

type RelayResult struct {
	StatusCode          int
	Header              http.Header
	Body                []byte
	Usage               vos.Usage
	FirstResponseTimeMs int64
	Timing              UpstreamTiming
	StreamStarted       bool
	StreamTerminal      string
	StreamTerminalError string
	StreamSynthesized   bool
}

type RelayAttempt struct {
	Attempt           int    `json:"attempt"`
	ProviderGuid      string `json:"providerGuid,omitempty"`
	ProviderName      string `json:"providerName,omitempty"`
	RequestedModel    string `json:"requestedModel,omitempty"`
	UpstreamModel     string `json:"upstreamModel,omitempty"`
	StatusCode        int    `json:"statusCode,omitempty"`
	Stage             string `json:"stage"`
	Status            string `json:"status"`
	Error             string `json:"error,omitempty"`
	DurationMs        int64  `json:"durationMs"`
	Retried           bool   `json:"retried"`
	StreamStarted     bool   `json:"streamStarted,omitempty"`
	UpstreamRequestID string `json:"upstreamRequestId,omitempty"`
}

type preparedRelay struct {
	Body        []byte
	ModelName   string
	Candidates  []domains.VendorMeta
	IsStream    bool
	Reservation *billingReservation
}

type RelayHTTPError struct {
	StatusCode int
	Message    string
	RetryAfter time.Duration
}

type billingReservation struct {
	AmountMicros   int64
	WalletRecordID uint
	Detail         QuotaCalculationDetail
}

func (e *RelayHTTPError) Error() string {
	return e.Message
}

// RelayHTTP is the single entry point used by handlers. It prepares the request
// once, then chooses buffered or live streaming delivery based on the original
// client payload.
func (s RelayService) RelayHTTP(c *gin.Context, token *domains.ApiToken, endpoint RelayEndpoint) (*RelayResult, bool, error) {
	if token == nil {
		return nil, false, &RelayHTTPError{StatusCode: http.StatusUnauthorized, Message: "token is invalid"}
	}
	release, err := UserConcurrencyServiceApp.Acquire(token.UserGuid, token.Guid)
	if err != nil {
		return nil, false, err
	}
	defer release()

	prepared, err := s.prepareRelay(c, token, endpoint)
	if err != nil {
		return nil, false, err
	}
	if prepared.IsStream {
		return nil, true, s.relayStream(c, token, endpoint, prepared)
	}
	result, err := s.relayBuffered(c, token, endpoint, prepared)
	return result, false, err
}

func (s RelayService) prepareRelay(c *gin.Context, token *domains.ApiToken, endpoint RelayEndpoint) (*preparedRelay, error) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return nil, &RelayHTTPError{StatusCode: http.StatusRequestEntityTooLarge, Message: "request body is too large"}
		}
		return nil, err
	}
	modelName := extractModelName(c, endpoint, body)
	if strings.TrimSpace(modelName) == "" {
		modelName = endpoint.DefaultModel
	}
	if strings.TrimSpace(modelName) == "" {
		return nil, errors.New("model is required")
	}
	if err := ValidateNoPrivateURLs(body, c.GetHeader("Content-Type")); err != nil {
		return nil, &RelayHTTPError{StatusCode: http.StatusBadRequest, Message: err.Error()}
	}
	if err := ValidateSensitiveWords(body); err != nil {
		return nil, err
	}
	if err := TokenServiceApp.CheckModel(token, modelName); err != nil {
		return nil, err
	}
	candidates, err := ProviderServiceApp.FindCandidatesForEndpointAndType(modelName, token.Group, endpoint.Format, endpoint.UpstreamPath)
	if err != nil {
		return nil, fmt.Errorf("no available provider for model %s", modelName)
	}
	if err := checkModelRateLimit(token, modelName); err != nil {
		return nil, err
	}
	candidates = ProviderServiceApp.ApplyAffinity(token.Guid, modelName, candidates)
	var reservation *billingReservation
	if !endpoint.NoBilling {
		reservation, err = s.preauthorizeCost(token, endpoint, modelName, body)
		if err != nil {
			return nil, err
		}
	}
	return &preparedRelay{
		Body:        body,
		ModelName:   modelName,
		Candidates:  candidates,
		IsStream:    isStreamRequest(body),
		Reservation: reservation,
	}, nil
}

func (s RelayService) relayBuffered(c *gin.Context, token *domains.ApiToken, endpoint RelayEndpoint, prepared *preparedRelay) (*RelayResult, error) {
	start := time.Now()
	var provider *domains.VendorMeta
	var result *RelayResult
	var err error
	attempts := 0
	routeAttempts := make([]RelayAttempt, 0, len(prepared.Candidates))
	var circuitRetryAfter time.Duration
	for i := range prepared.Candidates {
		current := prepared.Candidates[i]
		permit, retryAfter, available := ProviderCircuitBreakerApp.TryAcquire(current.Guid, prepared.ModelName, endpoint.UpstreamPath)
		if !available {
			if retryAfter > 0 && (circuitRetryAfter <= 0 || retryAfter < circuitRetryAfter) {
				circuitRetryAfter = retryAfter
			}
			routeAttempts = append(routeAttempts, newCircuitSkippedAttempt(&current, prepared.ModelName, retryAfter))
			continue
		}
		attempts++
		forwardBody, upstreamPath := buildUpstreamRequest(&current, prepared.ModelName, endpoint, prepared.Body, c.GetHeader("Content-Type"))
		provider = &current
		attemptStart := time.Now()
		result, err = s.forward(c.Request.Context(), &current, endpoint.Method, upstreamPath, forwardBody, c.Request.Header, c.Request.URL.RawQuery)
		if result != nil {
			result.Timing.AttemptCount = attempts
		}
		outcome := classifyProviderCircuitOutcome(c.Request.Context(), result, err, time.Now())
		ProviderCircuitBreakerApp.Record(permit, outcome)
		if shouldForgetProviderAffinity(outcome, result, err) {
			ProviderServiceApp.ForgetAffinity(token.Guid, prepared.ModelName, current.Guid)
		}
		maybeAutoDisableProvider(&current, result)
		willRetry := (err != nil || (result != nil && shouldRetryRelayStatus(result.StatusCode))) && i < len(prepared.Candidates)-1
		routeAttempts = append(routeAttempts, newRelayAttempt(attempts, &current, prepared.ModelName, result, err, time.Since(attemptStart), willRetry))
		if err != nil && willRetry {
			continue
		}
		if err == nil && result != nil && shouldRetryRelayStatus(result.StatusCode) && willRetry {
			continue
		}
		break
	}
	useTime := time.Since(start).Milliseconds()
	if attempts == 0 {
		err = providerCircuitUnavailableError(circuitRetryAfter)
		s.cancelReservation(token, prepared.Reservation, err.Error())
		provider = firstRelayProvider(prepared.Candidates)
		_ = LogServiceApp.Create(buildUsageLog(c, token, provider, prepared.ModelName, vos.Usage{}, 0, useTime, 0, usageLogTiming(nil, prepared.Body, attempts), prepared.IsStream, "error", err.Error(), prepared.Body, "", routeAttempts))
		return nil, err
	}
	status := "success"
	content := ""
	if err != nil {
		status = "error"
		content = relayFailureMessage(result, err)
		s.cancelReservation(token, prepared.Reservation, content)
		_ = LogServiceApp.Create(buildUsageLog(c, token, provider, prepared.ModelName, vos.Usage{}, 0, useTime, 0, usageLogTiming(result, prepared.Body, attempts), prepared.IsStream, status, content, prepared.Body, "", routeAttempts, result))
		return nil, err
	}
	if result.StatusCode >= http.StatusBadRequest {
		status = "error"
		content = relayFailureMessage(result, nil)
		s.cancelReservation(token, prepared.Reservation, content)
		_ = LogServiceApp.Create(buildUsageLog(c, token, provider, prepared.ModelName, result.Usage, 0, useTime, firstResponseTime(result), usageLogTiming(result, prepared.Body, attempts), prepared.IsStream, status, content, prepared.Body, extractUpstreamRequestID(result), routeAttempts, result))
		return result, nil
	}
	if provider != nil {
		ProviderServiceApp.RememberAffinity(token.Guid, prepared.ModelName, provider.Guid)
	}
	quota := calculateFinalQuota(prepared.ModelName, token.Group, result.Usage, prepared.Body, 0)
	if !endpoint.NoBilling {
		detail := PricingServiceApp.CalculateQuotaDetail(prepared.ModelName, token.Group, result.Usage, estimateQuotaFromBody(prepared.Body))
		if err := s.settleCost(token, prepared.Reservation, CostToAmountMicros(detail.FinalCost), detail); err != nil {
			status = "error"
			content = err.Error()
			s.cancelReservation(token, prepared.Reservation, content)
			_ = LogServiceApp.Create(buildUsageLog(c, token, provider, prepared.ModelName, result.Usage, 0, useTime, firstResponseTime(result), usageLogTiming(result, prepared.Body, attempts), prepared.IsStream, status, content, prepared.Body, extractUpstreamRequestID(result), routeAttempts, result))
			return nil, err
		}
	} else {
		quota = 0
	}
	_ = LogServiceApp.Create(buildUsageLog(c, token, provider, prepared.ModelName, result.Usage, quota, useTime, firstResponseTime(result), usageLogTiming(result, prepared.Body, attempts), prepared.IsStream, status, content, prepared.Body, extractUpstreamRequestID(result), routeAttempts, result))
	return result, nil
}

func (s RelayService) relayStream(c *gin.Context, token *domains.ApiToken, endpoint RelayEndpoint, prepared *preparedRelay) error {
	start := time.Now()
	var provider *domains.VendorMeta
	var result *RelayResult
	var err error
	attempts := 0
	routeAttempts := make([]RelayAttempt, 0, len(prepared.Candidates))
	var circuitRetryAfter time.Duration
	for i := range prepared.Candidates {
		current := prepared.Candidates[i]
		permit, retryAfter, available := ProviderCircuitBreakerApp.TryAcquire(current.Guid, prepared.ModelName, endpoint.UpstreamPath)
		if !available {
			if retryAfter > 0 && (circuitRetryAfter <= 0 || retryAfter < circuitRetryAfter) {
				circuitRetryAfter = retryAfter
			}
			routeAttempts = append(routeAttempts, newCircuitSkippedAttempt(&current, prepared.ModelName, retryAfter))
			continue
		}
		attempts++
		forwardBody, upstreamPath := buildUpstreamRequest(&current, prepared.ModelName, endpoint, prepared.Body, c.GetHeader("Content-Type"))
		provider = &current
		attemptStart := time.Now()
		result, err = s.forwardStream(c, &current, endpoint.Method, upstreamPath, forwardBody, c.Request.Header, c.Request.URL.RawQuery, i < len(prepared.Candidates)-1)
		if result != nil {
			result.Timing.AttemptCount = attempts
		}
		outcome := classifyProviderCircuitOutcome(c.Request.Context(), result, err, time.Now())
		ProviderCircuitBreakerApp.Record(permit, outcome)
		if shouldForgetProviderAffinity(outcome, result, err) {
			ProviderServiceApp.ForgetAffinity(token.Guid, prepared.ModelName, current.Guid)
		}
		maybeAutoDisableProvider(&current, result)
		willRetry := i < len(prepared.Candidates)-1 && ((err != nil && canRetryStreamAttempt(result)) || (err == nil && result != nil && shouldRetryRelayStatus(result.StatusCode)))
		routeAttempts = append(routeAttempts, newRelayAttempt(attempts, &current, prepared.ModelName, result, err, time.Since(attemptStart), willRetry))
		if err != nil && willRetry {
			continue
		}
		if err == nil && result != nil && shouldRetryRelayStatus(result.StatusCode) && willRetry {
			continue
		}
		break
	}

	useTime := time.Since(start).Milliseconds()
	if attempts == 0 {
		err = providerCircuitUnavailableError(circuitRetryAfter)
		s.cancelReservation(token, prepared.Reservation, err.Error())
		provider = firstRelayProvider(prepared.Candidates)
		_ = LogServiceApp.Create(buildUsageLog(c, token, provider, prepared.ModelName, vos.Usage{}, 0, useTime, 0, usageLogTiming(nil, prepared.Body, attempts), prepared.IsStream, "error", err.Error(), prepared.Body, "", routeAttempts))
		return err
	}
	if err != nil {
		s.cancelReservation(token, prepared.Reservation, err.Error())
		usage := vos.Usage{}
		firstResponseMs := int64(0)
		upstreamRequestID := ""
		if result != nil {
			usage = result.Usage
			firstResponseMs = firstResponseTime(result)
			upstreamRequestID = extractUpstreamRequestID(result)
		}
		_ = LogServiceApp.Create(buildUsageLog(c, token, provider, prepared.ModelName, usage, 0, useTime, firstResponseMs, usageLogTiming(result, prepared.Body, attempts), prepared.IsStream, "error", relayFailureMessage(result, err), prepared.Body, upstreamRequestID, routeAttempts, result))
		return err
	}
	if result == nil {
		err = errors.New("upstream response is empty")
		s.cancelReservation(token, prepared.Reservation, err.Error())
		_ = LogServiceApp.Create(buildUsageLog(c, token, provider, prepared.ModelName, vos.Usage{}, 0, useTime, 0, usageLogTiming(result, prepared.Body, attempts), prepared.IsStream, "error", err.Error(), prepared.Body, "", routeAttempts))
		return err
	}
	if result.StatusCode >= http.StatusBadRequest {
		if !result.StreamStarted {
			writeBufferedStreamResult(c, result)
		}
		content := relayFailureMessage(result, nil)
		s.cancelReservation(token, prepared.Reservation, content)
		_ = LogServiceApp.Create(buildUsageLog(c, token, provider, prepared.ModelName, result.Usage, 0, useTime, firstResponseTime(result), usageLogTiming(result, prepared.Body, attempts), prepared.IsStream, "error", content, prepared.Body, extractUpstreamRequestID(result), routeAttempts, result))
		return nil
	}
	if result.StreamSynthesized {
		content := synthesizedStreamLogContent(result)
		s.cancelReservation(token, prepared.Reservation, content)
		_ = LogServiceApp.Create(buildUsageLog(c, token, provider, prepared.ModelName, vos.Usage{}, 0, useTime, firstResponseTime(result), usageLogTiming(result, prepared.Body, attempts), prepared.IsStream, "synthesized", content, prepared.Body, extractUpstreamRequestID(result), routeAttempts, result))
		return nil
	}
	if provider != nil {
		ProviderServiceApp.RememberAffinity(token.Guid, prepared.ModelName, provider.Guid)
	}
	quota := calculateFinalQuota(prepared.ModelName, token.Group, result.Usage, prepared.Body, 0)
	if endpoint.NoBilling {
		quota = 0
	} else {
		detail := PricingServiceApp.CalculateQuotaDetail(prepared.ModelName, token.Group, result.Usage, estimateQuotaFromBody(prepared.Body))
		if err := s.settleCost(token, prepared.Reservation, CostToAmountMicros(detail.FinalCost), detail); err != nil {
			// The stream may already be on the wire, so settlement failures are
			// recorded in logs instead of trying to replace the response body.
			s.keepReservedCost(token, prepared.Reservation, detail)
			_ = LogServiceApp.Create(buildUsageLog(c, token, provider, prepared.ModelName, result.Usage, 0, useTime, firstResponseTime(result), usageLogTiming(result, prepared.Body, attempts), prepared.IsStream, "error", err.Error(), prepared.Body, extractUpstreamRequestID(result), routeAttempts, result))
			return nil
		}
	}
	_ = LogServiceApp.Create(buildUsageLog(c, token, provider, prepared.ModelName, result.Usage, quota, useTime, firstResponseTime(result), usageLogTiming(result, prepared.Body, attempts), prepared.IsStream, "success", "", prepared.Body, extractUpstreamRequestID(result), routeAttempts, result))
	return nil
}

func checkModelRateLimit(token *domains.ApiToken, modelName string) error {
	limit := OptionServiceApp.Int64("relay.model_rate_limit_count", 0)
	windowSeconds := OptionServiceApp.Int64("relay.model_rate_limit_window_seconds", 60)
	if token == nil || !OptionServiceApp.Bool("relay.model_rate_limit_enabled", limit > 0) || limit <= 0 || windowSeconds <= 0 {
		return nil
	}
	key := token.Guid + ":" + strings.TrimSpace(modelName)
	ok, retryAfter := RateLimitServiceApp.Allow(key, limit, time.Duration(windowSeconds)*time.Second)
	if ok {
		return nil
	}
	message := "rate limit exceeded"
	if retryAfter > 0 {
		seconds := int64((retryAfter + time.Second - 1) / time.Second)
		message = fmt.Sprintf("rate limit exceeded, retry after %ds", seconds)
	}
	return &RelayHTTPError{
		StatusCode: http.StatusTooManyRequests,
		Message:    message,
		RetryAfter: retryAfter,
	}
}

func shouldRetryRelayStatus(statusCode int) bool {
	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden || statusCode == http.StatusNotFound || statusCode == http.StatusRequestTimeout || statusCode == http.StatusConflict || statusCode == http.StatusTooManyRequests {
		return true
	}
	return statusCode >= http.StatusInternalServerError
}

func firstRelayProvider(candidates []domains.VendorMeta) *domains.VendorMeta {
	if len(candidates) == 0 {
		return nil
	}
	provider := candidates[0]
	return &provider
}

func relayProviderName(provider *domains.VendorMeta) string {
	if provider == nil {
		return ""
	}
	if name := strings.TrimSpace(provider.DisplayName); name != "" {
		return name
	}
	return strings.TrimSpace(provider.VendorName)
}

func newCircuitSkippedAttempt(provider *domains.VendorMeta, modelName string, retryAfter time.Duration) RelayAttempt {
	message := "provider circuit breaker is cooling down"
	if retryAfter > 0 {
		message += ", retry after " + retryAfter.Round(time.Second).String()
	}
	return RelayAttempt{
		ProviderGuid:   provider.Guid,
		ProviderName:   relayProviderName(provider),
		RequestedModel: modelName,
		UpstreamModel:  ProviderServiceApp.MapModel(provider, modelName),
		Stage:          "circuit_breaker",
		Status:         "skipped",
		Error:          message,
	}
}

func newRelayAttempt(attempt int, provider *domains.VendorMeta, modelName string, result *RelayResult, err error, duration time.Duration, retried bool) RelayAttempt {
	item := RelayAttempt{
		Attempt:        attempt,
		ProviderGuid:   provider.Guid,
		ProviderName:   relayProviderName(provider),
		RequestedModel: modelName,
		UpstreamModel:  ProviderServiceApp.MapModel(provider, modelName),
		DurationMs:     duration.Milliseconds(),
		Retried:        retried,
		Stage:          "completed",
		Status:         "success",
	}
	if result != nil {
		item.StatusCode = result.StatusCode
		item.StreamStarted = result.StreamStarted
		item.UpstreamRequestID = extractUpstreamRequestID(result)
	}
	if err != nil {
		item.Stage = relayFailureStage(result, err)
		item.Status = "failed"
		item.Error = relayFailureMessage(result, err)
		return item
	}
	if result == nil {
		item.Stage = "upstream_response"
		item.Status = "failed"
		item.Error = "upstream response is empty"
		return item
	}
	if result.StreamSynthesized {
		item.Stage = "stream_terminal"
		item.Status = "synthesized"
		item.Error = synthesizedStreamLogContent(result)
		return item
	}
	if result.StatusCode >= http.StatusBadRequest {
		item.Stage = "upstream_response"
		item.Status = "failed"
		item.Error = relayFailureMessage(result, nil)
	}
	return item
}

func relayFailureStage(result *RelayResult, err error) string {
	if isDownstreamStreamWriteError(err) || errors.Is(err, context.Canceled) {
		return "client_disconnected"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "upstream_timeout"
	}
	if result != nil {
		if result.StreamTerminal != "" || result.StreamStarted {
			return "upstream_stream"
		}
		if result.StatusCode > 0 {
			return "upstream_response_read"
		}
	}
	return "upstream_connect"
}

func relayFailureMessage(result *RelayResult, err error) string {
	if result != nil && result.StatusCode >= http.StatusBadRequest {
		message := extractUpstreamErrorMessage(result.Body)
		if message == "" {
			message = http.StatusText(result.StatusCode)
		}
		if message == "" {
			message = "unknown upstream error"
		}
		return limitRelayLogText(fmt.Sprintf("upstream returned HTTP %d: %s", result.StatusCode, message), 1000)
	}
	if result != nil && result.StreamTerminalError != "" {
		return limitRelayLogText("upstream stream failed: "+result.StreamTerminalError, 1000)
	}
	if err == nil {
		return "upstream request failed"
	}
	message := sanitizeRelayLogText(err.Error())
	switch {
	case isDownstreamStreamWriteError(err), errors.Is(err, context.Canceled):
		return limitRelayLogText("client disconnected: "+message, 1000)
	case errors.Is(err, context.DeadlineExceeded):
		return limitRelayLogText("upstream request timed out: "+message, 1000)
	default:
		return limitRelayLogText("upstream request failed: "+message, 1000)
	}
}

func extractUpstreamErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var payload any
	if json.Unmarshal(body, &payload) == nil {
		if message := findUpstreamErrorMessage(payload); message != "" {
			return limitRelayLogText(message, 800)
		}
	}
	return limitRelayLogText(string(body), 800)
}

func findUpstreamErrorMessage(value any) string {
	switch item := value.(type) {
	case map[string]any:
		for _, key := range []string{"error", "message", "detail", "reason"} {
			child, ok := item[key]
			if !ok {
				continue
			}
			if text, ok := child.(string); ok && strings.TrimSpace(text) != "" {
				return text
			}
			if message := findUpstreamErrorMessage(child); message != "" {
				return message
			}
		}
		for _, key := range []string{"code", "type"} {
			if text, ok := item[key].(string); ok && strings.TrimSpace(text) != "" {
				return text
			}
		}
	case []any:
		for _, child := range item {
			if message := findUpstreamErrorMessage(child); message != "" {
				return message
			}
		}
	}
	return ""
}

var relaySecretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(authorization\s*[:=]\s*bearer\s+)[^\s,;]+`),
	regexp.MustCompile(`(?i)((?:api[_-]?key|access[_-]?token|key)\s*[:=]\s*)[^&\s,;]+`),
}

func sanitizeRelayLogText(value string) string {
	value = strings.TrimSpace(value)
	for _, pattern := range relaySecretPatterns {
		value = pattern.ReplaceAllString(value, `${1}***`)
	}
	return strings.Join(strings.Fields(value), " ")
}

func limitRelayLogText(value string, limit int) string {
	value = sanitizeRelayLogText(value)
	if limit > 0 && len(value) > limit {
		return value[:limit]
	}
	return value
}

func canRetryStreamAttempt(result *RelayResult) bool {
	return result == nil || !result.StreamStarted
}

func shouldForgetProviderAffinity(outcome providerCircuitOutcome, result *RelayResult, err error) bool {
	if outcome.Kind == providerCircuitIgnored {
		return false
	}
	if outcome.Kind == providerCircuitHealthy && isUpstreamResponseLimitError(err) {
		return false
	}
	if result != nil && result.StreamSynthesized {
		return true
	}
	if err != nil {
		return true
	}
	return result != nil && shouldRetryRelayStatus(result.StatusCode)
}

func providerCircuitUnavailableError(retryAfter time.Duration) error {
	message := "all available providers are cooling down"
	if retryAfter > 0 {
		message = fmt.Sprintf("all available providers are cooling down, retry after %s", retryAfter.Round(time.Second))
	}
	return &RelayHTTPError{
		StatusCode: http.StatusServiceUnavailable,
		Message:    message,
		RetryAfter: retryAfter,
	}
}

func maybeAutoDisableProvider(provider *domains.VendorMeta, result *RelayResult) {
	if provider == nil || result == nil {
		return
	}
	if result.StatusCode != http.StatusUnauthorized && result.StatusCode != http.StatusForbidden {
		return
	}
	reason := fmt.Sprintf("auto disabled after upstream status %d", result.StatusCode)
	if len(result.Body) > 0 {
		body := string(result.Body)
		if len(body) > 180 {
			body = body[:180]
		}
		reason += ": " + body
	}
	_ = ProviderServiceApp.AutoDisable(provider.Guid, reason)
}

func buildUpstreamRequest(provider *domains.VendorMeta, modelName string, endpoint RelayEndpoint, body []byte, contentType string) ([]byte, string) {
	upstreamModel := ProviderServiceApp.MapModel(provider, modelName)
	if endpoint.ModelFromPath {
		return body, rewriteModelInPath(endpoint.UpstreamPath, upstreamModel)
	}
	forwardBody := rewriteBodyModel(body, upstreamModel, contentType)
	if endpoint.Format == constants.ProviderTypeOpenAI {
		forwardBody = ensureOpenAIStreamUsage(forwardBody, contentType, endpoint.UpstreamPath)
	}
	return forwardBody, endpoint.UpstreamPath
}

func (s RelayService) forward(ctx context.Context, provider *domains.VendorMeta, method string, upstreamPath string, body []byte, incoming http.Header, rawQuery string) (*RelayResult, error) {
	baseURL := strings.TrimRight(provider.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL(provider.Type)
	}
	targetURL := joinProviderEndpoint(baseURL, upstreamPath)
	if rawQuery != "" && provider.Type != constants.ProviderTypeGemini {
		targetURL += "?" + rawQuery
	}
	if provider.Type == constants.ProviderTypeGemini {
		targetURL = attachGeminiKey(targetURL, provider.Key, rawQuery)
	}
	targetURL = applyParamOverride(targetURL, provider.ParamOverride)
	req, err := http.NewRequestWithContext(ctx, method, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	copyForwardHeaders(req.Header, incoming)
	setupAuthHeaders(req.Header, provider)
	applyHeaderOverride(req.Header, provider.HeaderOverride)
	client, err := s.clientForProvider(provider)
	if err != nil {
		return nil, err
	}
	requestStart := time.Now()
	trace := newUpstreamRequestTrace(requestStart, int64(len(body)))
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace.ClientTrace()))
	resp, err := client.Do(req)
	if err != nil {
		return &RelayResult{Timing: trace.Snapshot(time.Since(requestStart))}, err
	}
	headerResponseTimeMs := time.Since(requestStart).Milliseconds()
	defer resp.Body.Close()
	respBody, err := readLimitedUpstreamBody(resp)
	if err != nil {
		return &RelayResult{StatusCode: resp.StatusCode, Header: resp.Header.Clone(), Timing: trace.Snapshot(time.Since(requestStart))}, err
	}
	timing := trace.Snapshot(time.Since(requestStart))
	if timing.ResponseHeaderTimeMs <= 0 {
		timing.ResponseHeaderTimeMs = headerResponseTimeMs
	}
	return &RelayResult{
		StatusCode:          resp.StatusCode,
		Header:              resp.Header.Clone(),
		Body:                respBody,
		Usage:               parseUsage(respBody, resp.Header.Get("Content-Type")),
		FirstResponseTimeMs: headerResponseTimeMs,
		Timing:              timing,
	}, nil
}

func (s RelayService) forwardStream(c *gin.Context, provider *domains.VendorMeta, method string, upstreamPath string, body []byte, incoming http.Header, rawQuery string, canRetry bool) (*RelayResult, error) {
	baseURL := strings.TrimRight(provider.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL(provider.Type)
	}
	targetURL := joinProviderEndpoint(baseURL, upstreamPath)
	if rawQuery != "" && provider.Type != constants.ProviderTypeGemini {
		targetURL += "?" + rawQuery
	}
	if provider.Type == constants.ProviderTypeGemini {
		targetURL = attachGeminiKey(targetURL, provider.Key, rawQuery)
	}
	targetURL = applyParamOverride(targetURL, provider.ParamOverride)
	req, err := http.NewRequestWithContext(c.Request.Context(), method, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	copyForwardHeaders(req.Header, incoming)
	setupAuthHeaders(req.Header, provider)
	applyHeaderOverride(req.Header, provider.HeaderOverride)
	client, err := s.streamClientForProvider(provider)
	if err != nil {
		return nil, err
	}
	requestStart := time.Now()
	trace := newUpstreamRequestTrace(requestStart, int64(len(body)))
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace.ClientTrace()))
	resp, err := client.Do(req)
	if err != nil {
		return &RelayResult{Timing: trace.Snapshot(time.Since(requestStart))}, err
	}
	headerResponseTimeMs := time.Since(requestStart).Milliseconds()
	defer resp.Body.Close()
	responseLimit := maxUpstreamResponseBytes()

	if resp.StatusCode >= http.StatusBadRequest {
		willRetry := canRetry && shouldRetryRelayStatus(resp.StatusCode)
		respBody, readErr := readLimitedUpstreamBody(resp)
		if readErr != nil {
			return &RelayResult{StatusCode: resp.StatusCode, Header: resp.Header.Clone(), Timing: trace.Snapshot(time.Since(requestStart))}, readErr
		}
		if !willRetry {
			copyResponseHeaders(c.Writer.Header(), resp.Header)
			c.Data(resp.StatusCode, contentTypeOrJSON(resp.Header), respBody)
		}
		timing := trace.Snapshot(time.Since(requestStart))
		if timing.ResponseHeaderTimeMs <= 0 {
			timing.ResponseHeaderTimeMs = headerResponseTimeMs
		}
		return &RelayResult{
			StatusCode:          resp.StatusCode,
			Header:              resp.Header.Clone(),
			Body:                respBody,
			Usage:               parseUsage(respBody, resp.Header.Get("Content-Type")),
			FirstResponseTimeMs: headerResponseTimeMs,
			Timing:              timing,
			StreamStarted:       !willRetry,
		}, nil
	}

	tracker := &streamUsageTracker{}
	firstResponseTimeMs := int64(0)
	streamStarted := false
	requireResponsesTerminal := strings.TrimSpace(upstreamPath) == "/v1/responses"
	responsesEOFTerminalPolicy := ""
	if requireResponsesTerminal {
		responsesEOFTerminalPolicy = responsesStreamEOFTerminalPolicy()
	}
	if responseLimit > 0 && resp.ContentLength > responseLimit {
		timing := trace.Snapshot(time.Since(requestStart))
		if timing.ResponseHeaderTimeMs <= 0 {
			timing.ResponseHeaderTimeMs = headerResponseTimeMs
		}
		return &RelayResult{
			StatusCode:          resp.StatusCode,
			Header:              resp.Header.Clone(),
			FirstResponseTimeMs: headerResponseTimeMs,
			Timing:              timing,
		}, upstreamResponseTooLargeError(responseLimit)
	}
	copyResponseHeaders(c.Writer.Header(), resp.Header)
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Status(resp.StatusCode)
	c.Writer.Flush()
	streamStarted = true

	streamedResponseBytes := int64(0)
	readContext, cancelReads := context.WithCancel(c.Request.Context())
	defer cancelReads()
	reads := streamBodyReads(readContext, resp.Body)
	heartbeatInterval := s.streamHeartbeatInterval
	if heartbeatInterval <= 0 {
		heartbeatInterval = defaultStreamHeartbeatInterval
	}
	var heartbeat *time.Timer
	var heartbeatC <-chan time.Time
	if isEventStreamResponse(resp.Header) && heartbeatInterval > 0 {
		heartbeat = time.NewTimer(heartbeatInterval)
		defer heartbeat.Stop()
		heartbeatC = heartbeat.C
	}
streamLoop:
	for {
		select {
		case read, ok := <-reads:
			if !ok {
				if err := readContext.Err(); err != nil {
					return finishStreamRelayResult(resp, tracker, firstResponseTimeMs, headerResponseTimeMs, requestStart, trace, streamStarted), err
				}
				break streamLoop
			}
			if len(read.Chunk) > 0 {
				if responseLimit > 0 && int64(len(read.Chunk)) > responseLimit-streamedResponseBytes {
					return finishStreamRelayResult(resp, tracker, firstResponseTimeMs, headerResponseTimeMs, requestStart, trace, streamStarted), upstreamResponseTooLargeError(responseLimit)
				}
				streamedResponseBytes += int64(len(read.Chunk))
				if firstResponseTimeMs <= 0 {
					firstResponseTimeMs = time.Since(requestStart).Milliseconds()
				}
				tracker.Write(read.Chunk)
				if _, writeErr := c.Writer.Write(read.Chunk); writeErr != nil {
					return finishStreamRelayResult(resp, tracker, firstResponseTimeMs, headerResponseTimeMs, requestStart, trace, streamStarted), &downstreamStreamWriteError{err: writeErr}
				}
				c.Writer.Flush()
				resetStreamHeartbeat(heartbeat, heartbeatInterval)
				if requireResponsesTerminal && tracker.terminal != "" {
					break streamLoop
				}
			}
			if read.Err == io.EOF {
				if shouldSynthesizeResponsesTerminal(c, tracker, streamStarted, responsesEOFTerminalPolicy) {
					if writeErr := writeSynthesizedResponsesTerminalEvent(c, tracker, responsesEOFTerminalPolicy, "upstream EOF before response terminal event"); writeErr != nil {
						return finishStreamRelayResult(resp, tracker, firstResponseTimeMs, headerResponseTimeMs, requestStart, trace, streamStarted), &downstreamStreamWriteError{err: writeErr}
					}
				}
				break streamLoop
			}
			if read.Err != nil {
				if shouldSynthesizeResponsesTerminal(c, tracker, streamStarted, responsesEOFTerminalPolicy) {
					if writeErr := writeSynthesizedResponsesTerminalEvent(c, tracker, responsesEOFTerminalPolicy, "upstream stream error before response terminal event: "+read.Err.Error()); writeErr != nil {
						return finishStreamRelayResult(resp, tracker, firstResponseTimeMs, headerResponseTimeMs, requestStart, trace, streamStarted), &downstreamStreamWriteError{err: writeErr}
					}
					break streamLoop
				}
				return finishStreamRelayResult(resp, tracker, firstResponseTimeMs, headerResponseTimeMs, requestStart, trace, streamStarted), read.Err
			}
		case <-heartbeatC:
			if _, writeErr := io.WriteString(c.Writer, streamHeartbeatComment); writeErr != nil {
				return finishStreamRelayResult(resp, tracker, firstResponseTimeMs, headerResponseTimeMs, requestStart, trace, streamStarted), &downstreamStreamWriteError{err: writeErr}
			}
			c.Writer.Flush()
			resetStreamHeartbeat(heartbeat, heartbeatInterval)
		case <-readContext.Done():
			return finishStreamRelayResult(resp, tracker, firstResponseTimeMs, headerResponseTimeMs, requestStart, trace, streamStarted), readContext.Err()
		}
	}
	result := finishStreamRelayResult(resp, tracker, firstResponseTimeMs, headerResponseTimeMs, requestStart, trace, streamStarted)
	if !requireResponsesTerminal {
		return result, nil
	}
	if responsesEOFTerminalPolicy == responsesEOFTerminalPolicyOff && result.StreamTerminal == "" {
		return result, nil
	}
	return result, responsesStreamTerminalError(result)
}

type streamBodyRead struct {
	Chunk []byte
	Err   error
}

func streamBodyReads(ctx context.Context, reader io.Reader) <-chan streamBodyRead {
	reads := make(chan streamBodyRead, 1)
	go func() {
		defer close(reads)
		buf := make([]byte, 32*1024)
		for {
			n, err := reader.Read(buf)
			if n == 0 && err == nil {
				continue
			}
			read := streamBodyRead{Err: err}
			if n > 0 {
				read.Chunk = append([]byte(nil), buf[:n]...)
			}
			select {
			case reads <- read:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	return reads
}

func isEventStreamResponse(header http.Header) bool {
	return strings.Contains(strings.ToLower(header.Get("Content-Type")), "text/event-stream")
}

func resetStreamHeartbeat(timer *time.Timer, interval time.Duration) {
	if timer == nil || interval <= 0 {
		return
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(interval)
}

func responsesStreamTerminalError(result *RelayResult) error {
	if result == nil {
		return errors.New("responses stream result is empty")
	}
	var err error
	switch result.StreamTerminal {
	case "response.completed":
		return nil
	case "response.failed":
		err = errors.New("responses stream failed")
	case "response.incomplete":
		err = errors.New("responses stream incomplete")
	default:
		err = errors.New("responses stream ended before terminal event")
	}
	if result.StreamTerminalError != "" {
		err = fmt.Errorf("%w: %s", err, result.StreamTerminalError)
	} else {
		result.StreamTerminalError = err.Error()
	}
	return err
}

func finishStreamRelayResult(resp *http.Response, tracker *streamUsageTracker, firstResponseTimeMs int64, headerResponseTimeMs int64, requestStart time.Time, trace *upstreamRequestTrace, streamStarted bool) *RelayResult {
	if firstResponseTimeMs <= 0 {
		firstResponseTimeMs = headerResponseTimeMs
	}
	timing := trace.Snapshot(time.Since(requestStart))
	if timing.ResponseHeaderTimeMs <= 0 {
		timing.ResponseHeaderTimeMs = headerResponseTimeMs
	}
	usage := tracker.Finish()
	return &RelayResult{
		StatusCode:          resp.StatusCode,
		Header:              resp.Header.Clone(),
		Usage:               usage,
		FirstResponseTimeMs: firstResponseTimeMs,
		Timing:              timing,
		StreamStarted:       streamStarted,
		StreamTerminal:      tracker.terminal,
		StreamTerminalError: tracker.terminalError,
		StreamSynthesized:   tracker.terminalSynthesized,
	}
}

const (
	responsesEOFTerminalPolicyCompleted  = "completed"
	responsesEOFTerminalPolicyIncomplete = "incomplete"
	responsesEOFTerminalPolicyFailed     = "failed"
	responsesEOFTerminalPolicyOff        = "off"
)

func responsesStreamEOFTerminalPolicy() string {
	return resolveResponsesStreamEOFTerminalPolicy(
		OptionServiceApp.Get("relay.responses_eof_terminal_policy", ""),
		OptionServiceApp.Get("relay.responses_synthesize_completed_on_eof", ""),
	)
}

func resolveResponsesStreamEOFTerminalPolicy(configuredPolicy string, legacyEnabled string) string {
	if policy, ok := normalizeResponsesEOFTerminalPolicy(configuredPolicy); ok {
		return policy
	}
	switch strings.ToLower(strings.TrimSpace(legacyEnabled)) {
	case "1", "true", "yes", "on":
		return responsesEOFTerminalPolicyIncomplete
	}
	return responsesEOFTerminalPolicyOff
}

func normalizeResponsesEOFTerminalPolicy(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case responsesEOFTerminalPolicyCompleted:
		return responsesEOFTerminalPolicyCompleted, true
	case responsesEOFTerminalPolicyIncomplete:
		return responsesEOFTerminalPolicyIncomplete, true
	case responsesEOFTerminalPolicyFailed:
		return responsesEOFTerminalPolicyFailed, true
	case responsesEOFTerminalPolicyOff, "none", "disabled", "false":
		return responsesEOFTerminalPolicyOff, true
	default:
		return "", false
	}
}

func responsesEOFTerminalEvent(policy string) string {
	switch policy {
	case responsesEOFTerminalPolicyCompleted:
		return "response.completed"
	case responsesEOFTerminalPolicyFailed:
		return "response.failed"
	case responsesEOFTerminalPolicyIncomplete:
		return "response.incomplete"
	default:
		return ""
	}
}

func shouldSynthesizeResponsesTerminal(c *gin.Context, tracker *streamUsageTracker, streamStarted bool, policy string) bool {
	if responsesEOFTerminalEvent(policy) == "" || !streamStarted || tracker == nil || tracker.terminal != "" {
		return false
	}
	if c != nil && c.Request != nil && c.Request.Context().Err() != nil {
		return false
	}
	return true
}

func synthesizedStreamLogContent(result *RelayResult) string {
	reason := ""
	if result != nil {
		reason = strings.TrimSpace(result.StreamTerminalError)
	}
	if reason == "" {
		reason = "upstream stream ended before response terminal event"
	}
	return "Responses 终止事件已补齐：" + reason
}

func writeSynthesizedResponsesTerminalEvent(c *gin.Context, tracker *streamUsageTracker, policy string, reason string) error {
	if c == nil || tracker == nil {
		return errors.New("stream context is empty")
	}
	eventType := responsesEOFTerminalEvent(policy)
	if eventType == "" {
		return errors.New("responses EOF terminal policy is disabled")
	}
	chunk := synthesizedResponsesTerminalEvent(tracker, eventType, reason, time.Now())
	if _, err := c.Writer.Write(chunk); err != nil {
		return err
	}
	c.Writer.Flush()
	tracker.Write(chunk)
	tracker.terminal = eventType
	tracker.terminalSynthesized = true
	tracker.terminalError = strings.TrimSpace(reason)
	return nil
}

func synthesizedResponsesTerminalEvent(tracker *streamUsageTracker, eventType string, reason string, now time.Time) []byte {
	response := synthesizedResponsesTerminalResponse(tracker, eventType, reason, now)
	payload := map[string]any{
		"type":     eventType,
		"response": response,
	}
	if tracker != nil && tracker.hasSequenceNumber {
		payload["sequence_number"] = tracker.sequenceNumber + 1
	}
	data, err := json.Marshal(payload)
	if err != nil {
		data = []byte(fmt.Sprintf(`{"type":%q,"response":{"status":%q,"output":[]}}`, eventType, responsesTerminalStatus(eventType)))
	}
	return []byte("\n\nevent: " + eventType + "\ndata: " + string(data) + "\n\n")
}

func synthesizedResponsesTerminalResponse(tracker *streamUsageTracker, eventType string, reason string, now time.Time) map[string]any {
	response := map[string]any{}
	if tracker != nil && len(tracker.responseSnapshot) > 0 {
		_ = json.Unmarshal(tracker.responseSnapshot, &response)
	}
	if response == nil {
		response = map[string]any{}
	}
	if _, ok := response["id"]; !ok {
		response["id"] = fmt.Sprintf("resp_synth_%d", now.UnixNano())
	}
	if _, ok := response["object"]; !ok {
		response["object"] = "response"
	}
	if _, ok := response["created_at"]; !ok {
		response["created_at"] = now.Unix()
	}
	if _, ok := response["output"]; !ok {
		response["output"] = []any{}
	}
	status := responsesTerminalStatus(eventType)
	response["status"] = status
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "upstream stream ended before response terminal event"
	}
	switch eventType {
	case "response.incomplete":
		response["incomplete_details"] = map[string]any{
			"reason": "upstream_stream_interrupted",
		}
		response["metadata"] = mergeResponsesMetadata(response["metadata"], map[string]string{
			"navapi_terminal_reason": reason,
		})
	case "response.failed":
		response["error"] = map[string]any{
			"code":    "upstream_stream_interrupted",
			"message": reason,
		}
	}
	if tracker != nil && hasUsageTokens(tracker.usage) {
		response["usage"] = responsesUsagePayload(tracker.usage)
	}
	return response
}

func responsesTerminalStatus(eventType string) string {
	switch eventType {
	case "response.completed":
		return "completed"
	case "response.failed":
		return "failed"
	case "response.incomplete":
		return "incomplete"
	default:
		return ""
	}
}

func mergeResponsesMetadata(current any, values map[string]string) map[string]any {
	metadata := map[string]any{}
	if existing, ok := current.(map[string]any); ok {
		for key, value := range existing {
			metadata[key] = value
		}
	}
	for key, value := range values {
		if strings.TrimSpace(value) != "" {
			metadata[key] = value
		}
	}
	return metadata
}

func responsesUsagePayload(usage vos.Usage) map[string]any {
	inputTokens := usage.PromptTokens
	if inputTokens <= 0 {
		inputTokens = usage.InputTokens
	}
	outputTokens := usage.CompletionTokens
	if outputTokens <= 0 {
		outputTokens = usage.OutputTokens
	}
	totalTokens := usage.TotalTokens
	if totalTokens <= 0 {
		totalTokens = inputTokens + outputTokens
	}
	payload := map[string]any{
		"input_tokens":  inputTokens,
		"output_tokens": outputTokens,
		"total_tokens":  totalTokens,
	}
	if usage.CachedTokens > 0 {
		payload["input_tokens_details"] = map[string]any{
			"cached_tokens": usage.CachedTokens,
		}
	}
	return payload
}

func readLimitedUpstreamBody(resp *http.Response) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, errors.New("upstream response body is empty")
	}
	limit := maxUpstreamResponseBytes()
	if limit <= 0 {
		return io.ReadAll(resp.Body)
	}
	if resp.ContentLength > limit {
		return nil, upstreamResponseTooLargeError(limit)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, upstreamResponseTooLargeError(limit)
	}
	return body, nil
}

func maxUpstreamResponseBytes() int64 {
	return OptionServiceApp.Int64("relay.max_upstream_response_bytes", defaultRiskMaxUpstreamResponseBytes)
}

type upstreamResponseLimitError struct {
	limit int64
}

func (e *upstreamResponseLimitError) Error() string {
	return fmt.Sprintf("upstream response body exceeds %d bytes", e.limit)
}

func isUpstreamResponseLimitError(err error) bool {
	var limitErr *upstreamResponseLimitError
	return errors.As(err, &limitErr)
}

func upstreamResponseTooLargeError(limit int64) error {
	return &upstreamResponseLimitError{limit: limit}
}

func copyResponseHeaders(dst http.Header, src http.Header) {
	for key, values := range src {
		if key == "Content-Length" || key == "Transfer-Encoding" {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func writeBufferedStreamResult(c *gin.Context, result *RelayResult) {
	if c == nil || result == nil || c.Writer.Written() {
		return
	}
	copyResponseHeaders(c.Writer.Header(), result.Header)
	c.Data(result.StatusCode, contentTypeOrJSON(result.Header), result.Body)
	result.StreamStarted = true
}

func contentTypeOrJSON(header http.Header) string {
	if contentType := header.Get("Content-Type"); contentType != "" {
		return contentType
	}
	return "application/json"
}

func applyHeaderOverride(header http.Header, raw string) {
	if strings.TrimSpace(raw) == "" {
		return
	}
	values := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return
	}
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if value == "" {
			header.Del(key)
			continue
		}
		header.Set(key, value)
	}
}

func applyParamOverride(targetURL string, raw string) string {
	if strings.TrimSpace(raw) == "" {
		return targetURL
	}
	values := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return targetURL
	}
	u, err := url.Parse(targetURL)
	if err != nil {
		return targetURL
	}
	query := u.Query()
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if value == nil {
			query.Del(key)
			continue
		}
		query.Set(key, fmt.Sprint(value))
	}
	u.RawQuery = query.Encode()
	return u.String()
}

func (s RelayService) ensureBillableBalance(token *domains.ApiToken) error {
	if token == nil {
		return errors.New("token is required")
	}
	if token.UnlimitedBalance {
		return nil
	}
	if effectiveTokenBalanceAmountMicros(token) <= 0 {
		return errors.New("token balance is exhausted")
	}
	if err := UserWalletServiceApp.Ensure(TokenServiceApp.DB(), token.UserGuid); err != nil {
		return err
	}
	wallet, err := UserWalletServiceApp.Get(token.UserGuid)
	if err != nil {
		return err
	}
	if wallet.BalanceAmountMicros <= 0 {
		return errors.New("wallet balance is insufficient")
	}
	return nil
}

func (s RelayService) preauthorizeCost(token *domains.ApiToken, endpoint RelayEndpoint, modelName string, body []byte) (*billingReservation, error) {
	if token == nil {
		return nil, errors.New("token is required")
	}
	estimatedUsage := estimatePreauthorizeUsage(endpoint, body)
	detail := PricingServiceApp.CalculateQuotaDetail(modelName, token.Group, estimatedUsage, estimateQuotaFromBody(body))
	amountMicros := CostToAmountMicros(detail.FinalCost)
	if amountMicros <= 0 {
		return nil, relayBillingError(s.ensureBillableBalance(token))
	}
	var reservation *billingReservation
	err := TokenServiceApp.DB().Transaction(func(tx *gorm.DB) error {
		if err := TokenServiceApp.ConsumeAmount(tx, token.Id, amountMicros); err != nil {
			return err
		}
		record, err := UserWalletServiceApp.ReserveConsume(tx, WalletRecordInput{
			UserGuid:     token.UserGuid,
			Type:         domains.WalletRecordTypeConsume,
			Source:       domains.WalletSourceRelay,
			Title:        "API 消费预授权",
			AmountMicros: amountMicros,
			RequestCount: 1,
			TokenID:      token.Id,
			TokenGuid:    token.Guid,
			Meta:         marshalBillingMeta(detail, amountMicros),
		})
		if err != nil {
			return err
		}
		if record == nil {
			return errors.New("wallet reservation failed")
		}
		reservation = &billingReservation{
			AmountMicros:   amountMicros,
			WalletRecordID: record.Id,
			Detail:         detail,
		}
		return nil
	})
	if err != nil {
		return nil, relayBillingError(err)
	}
	return reservation, nil
}

func relayBillingError(err error) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	if strings.Contains(message, "balance") {
		return &RelayHTTPError{StatusCode: http.StatusPaymentRequired, Message: message}
	}
	return err
}

func (s RelayService) settleCost(token *domains.ApiToken, reservation *billingReservation, amountMicros int64, detail QuotaCalculationDetail) error {
	err := TokenServiceApp.DB().Transaction(func(tx *gorm.DB) error {
		reservedAmount := int64(0)
		walletRecordID := uint(0)
		if reservation != nil {
			reservedAmount = reservation.AmountMicros
			walletRecordID = reservation.WalletRecordID
		}
		if amountMicros > reservedAmount {
			if err := TokenServiceApp.ConsumeAmount(tx, token.Id, amountMicros-reservedAmount); err != nil {
				return err
			}
		} else if amountMicros < reservedAmount {
			if err := TokenServiceApp.RefundAmount(tx, token.Id, reservedAmount-amountMicros); err != nil {
				return err
			}
		}
		input := WalletRecordInput{
			UserGuid:     token.UserGuid,
			Type:         domains.WalletRecordTypeConsume,
			Source:       domains.WalletSourceRelay,
			Title:        "API 消费",
			AmountMicros: amountMicros,
			RequestCount: 1,
			TokenID:      token.Id,
			TokenGuid:    token.Guid,
			Meta:         marshalBillingMeta(detail, amountMicros),
		}
		if walletRecordID > 0 {
			return UserWalletServiceApp.FinalizeReservedConsume(tx, walletRecordID, input)
		}
		return UserWalletServiceApp.RecordConsume(tx, input)
	})
	if err != nil {
		return err
	}
	UserWalletServiceApp.NotifyBalanceReminderAsync(token.UserGuid, "API 调用消费后账户余额低于 10 元")
	return nil
}

func (s RelayService) cancelReservation(token *domains.ApiToken, reservation *billingReservation, reason string) {
	if token == nil || reservation == nil || reservation.AmountMicros <= 0 {
		return
	}
	_ = TokenServiceApp.DB().Transaction(func(tx *gorm.DB) error {
		if err := TokenServiceApp.RefundAmount(tx, token.Id, reservation.AmountMicros); err != nil {
			return err
		}
		return UserWalletServiceApp.CancelReservedConsume(tx, reservation.WalletRecordID, reason)
	})
}

func (s RelayService) keepReservedCost(token *domains.ApiToken, reservation *billingReservation, detail QuotaCalculationDetail) {
	if token == nil || reservation == nil || reservation.AmountMicros <= 0 {
		return
	}
	err := TokenServiceApp.DB().Transaction(func(tx *gorm.DB) error {
		return UserWalletServiceApp.FinalizeReservedConsume(tx, reservation.WalletRecordID, WalletRecordInput{
			UserGuid:     token.UserGuid,
			Type:         domains.WalletRecordTypeConsume,
			Source:       domains.WalletSourceRelay,
			Title:        "API 消费",
			AmountMicros: reservation.AmountMicros,
			RequestCount: 1,
			TokenID:      token.Id,
			TokenGuid:    token.Guid,
			Meta:         marshalBillingMeta(detail, reservation.AmountMicros),
		})
	})
	if err == nil {
		UserWalletServiceApp.NotifyBalanceReminderAsync(token.UserGuid, "API 调用消费后账户余额低于 10 元")
	}
}

func defaultBaseURL(providerType string) string {
	switch providerType {
	case constants.ProviderTypeAnthropic:
		return "https://api.anthropic.com"
	case constants.ProviderTypeGemini:
		return "https://generativelanguage.googleapis.com"
	default:
		return "https://api.openai.com"
	}
}

func setupAuthHeaders(header http.Header, provider *domains.VendorMeta) {
	key := strings.TrimSpace(ProviderServiceApp.NextKey(provider))
	switch provider.Type {
	case constants.ProviderTypeAnthropic:
		header.Set("x-api-key", key)
		if header.Get("anthropic-version") == "" {
			header.Set("anthropic-version", "2023-06-01")
		}
		header.Del("Authorization")
	case constants.ProviderTypeGemini:
		header.Del("Authorization")
	default:
		header.Set("Authorization", "Bearer "+key)
	}
}

func copyForwardHeaders(dst http.Header, src http.Header) {
	for key, values := range src {
		canonical := textproto.CanonicalMIMEHeaderKey(key)
		if !isAllowedForwardHeader(canonical) {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
	if dst.Get("Content-Type") == "" {
		dst.Set("Content-Type", "application/json")
	}
}

func isAllowedForwardHeader(key string) bool {
	switch {
	case strings.EqualFold(key, "Accept"):
		return true
	case strings.EqualFold(key, "Content-Type"):
		return true
	case strings.EqualFold(key, "Anthropic-Version"):
		return true
	case strings.EqualFold(key, "OpenAI-Beta"):
		return true
	default:
		return false
	}
}

func attachGeminiKey(targetURL string, key string, rawQuery string) string {
	u, err := url.Parse(targetURL)
	if err != nil {
		return targetURL
	}
	query := u.Query()
	if rawQuery != "" {
		incoming, _ := url.ParseQuery(rawQuery)
		for k, values := range incoming {
			for _, value := range values {
				query.Add(k, value)
			}
		}
	}
	if query.Get("key") == "" {
		query.Set("key", strings.TrimSpace(key))
	}
	u.RawQuery = query.Encode()
	return u.String()
}

func extractModelName(c *gin.Context, endpoint RelayEndpoint, body []byte) string {
	if endpoint.ModelFromPath {
		if model := modelFromPath(endpoint.UpstreamPath); model != "" {
			return model
		}
	}
	contentType := c.GetHeader("Content-Type")
	if strings.Contains(contentType, "multipart/form-data") {
		return extractMultipartModel(contentType, body)
	}
	var req vos.ModelRequest
	if err := json.Unmarshal(body, &req); err == nil {
		if req.Model != "" {
			return req.Model
		}
		if req.ModelName != "" {
			return req.ModelName
		}
	}
	return c.Param("model")
}

func extractMultipartModel(contentType string, body []byte) string {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil || params["boundary"] == "" {
		return ""
	}
	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	form, err := reader.ReadForm(32 << 20)
	if err != nil {
		return ""
	}
	defer form.RemoveAll()
	if values := form.Value["model"]; len(values) > 0 {
		return values[0]
	}
	return ""
}

func modelFromPath(upstreamPath string) string {
	re := regexp.MustCompile(`/models/([^:/]+)`)
	matches := re.FindStringSubmatch(upstreamPath)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func rewriteModelInPath(upstreamPath string, model string) string {
	if model == "" {
		return upstreamPath
	}
	re := regexp.MustCompile(`/models/([^:/]+)`)
	return re.ReplaceAllString(upstreamPath, "/models/"+model)
}

func rewriteBodyModel(body []byte, model string, contentType string) []byte {
	if model == "" || (contentType != "" && !strings.Contains(contentType, "application/json")) {
		return body
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}
	payload["model"] = model
	next, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return next
}

func ensureOpenAIStreamUsage(body []byte, contentType string, upstreamPath string) []byte {
	if contentType != "" && !strings.Contains(contentType, "application/json") {
		return body
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}
	if strings.TrimSpace(upstreamPath) == "/v1/responses" {
		changed := sanitizeResponsesInputMessageIDs(payload["input"])
		if _, exists := payload["stream_options"]; exists {
			delete(payload, "stream_options")
			changed = true
		}
		if !changed {
			return body
		}
		next, err := json.Marshal(payload)
		if err != nil {
			return body
		}
		return next
	}
	stream, ok := payload["stream"].(bool)
	if !ok || !stream {
		return body
	}
	options, _ := payload["stream_options"].(map[string]any)
	if options == nil {
		options = map[string]any{}
	}
	if include, exists := options["include_usage"].(bool); exists && include {
		return body
	}
	options["include_usage"] = true
	payload["stream_options"] = options
	next, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return next
}

func sanitizeResponsesInputMessageIDs(input any) bool {
	changed := false
	switch typed := input.(type) {
	case []any:
		for _, item := range typed {
			changed = sanitizeResponsesInputMessageID(item) || changed
		}
	case map[string]any:
		changed = sanitizeResponsesInputMessageID(typed)
	}
	return changed
}

func sanitizeResponsesInputMessageID(value any) bool {
	item, ok := value.(map[string]any)
	if !ok || !isResponsesInputMessage(item) {
		return false
	}
	idValue, exists := item["id"]
	if !exists {
		return false
	}
	id, ok := idValue.(string)
	if ok && strings.HasPrefix(id, "msg_") {
		return false
	}
	delete(item, "id")
	return true
}

func isResponsesInputMessage(item map[string]any) bool {
	itemType := strings.ToLower(strings.TrimSpace(fmt.Sprint(item["type"])))
	if itemType == "message" {
		return true
	}
	if itemType != "" && itemType != "<nil>" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(fmt.Sprint(item["role"]))) {
	case "user", "assistant", "system", "developer":
		return true
	default:
		return false
	}
}

func parseUsage(body []byte, contentType string) vos.Usage {
	if strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		if usage := parseStreamUsage(body); usage.TotalTokens > 0 || usage.PromptTokens > 0 || usage.CompletionTokens > 0 {
			return usage
		}
	}
	var payload struct {
		Usage    vos.Usage `json:"usage"`
		Response struct {
			Usage vos.Usage `json:"usage"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return vos.Usage{}
	}
	usage := payload.Usage
	if !hasUsageTokens(usage) {
		usage = payload.Response.Usage
	}
	return normalizeUsage(usage)
}

func normalizeUsage(usage vos.Usage) vos.Usage {
	if usage.PromptTokens == 0 {
		usage.PromptTokens = usage.InputTokens
	}
	if usage.CompletionTokens == 0 {
		usage.CompletionTokens = usage.OutputTokens
	}
	if usage.CachedTokens == 0 {
		usage.CachedTokens = usage.PromptTokensDetails.CachedTokens + usage.InputTokensDetails.CachedTokens
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	return usage
}

func hasUsageTokens(usage vos.Usage) bool {
	return usage.TotalTokens > 0 ||
		usage.PromptTokens > 0 ||
		usage.CompletionTokens > 0 ||
		usage.InputTokens > 0 ||
		usage.OutputTokens > 0
}

func parseStreamUsage(body []byte) vos.Usage {
	var usage vos.Usage
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		parsed := parseUsage([]byte(data), "application/json")
		if parsed.TotalTokens > 0 || parsed.PromptTokens > 0 || parsed.CompletionTokens > 0 {
			usage = parsed
		}
	}
	return usage
}

type streamUsageTracker struct {
	pending             string
	eventType           string
	usage               vos.Usage
	terminal            string
	terminalError       string
	terminalSynthesized bool
	responseSnapshot    json.RawMessage
	sequenceNumber      int64
	hasSequenceNumber   bool
}

// Write incrementally parses SSE "data:" lines while bytes are being proxied.
// This keeps streaming live without waiting to buffer the entire upstream body.
func (t *streamUsageTracker) Write(chunk []byte) {
	t.pending += string(chunk)
	for {
		idx := strings.IndexByte(t.pending, '\n')
		if idx < 0 {
			if len(t.pending) > 1<<20 {
				t.pending = t.pending[len(t.pending)-(1<<20):]
			}
			return
		}
		t.consumeLine(t.pending[:idx])
		t.pending = t.pending[idx+1:]
	}
}

func (t *streamUsageTracker) Finish() vos.Usage {
	if strings.TrimSpace(t.pending) != "" {
		t.consumeLine(t.pending)
		t.pending = ""
	}
	if t.terminal == "" && isResponsesTerminalEvent(t.eventType) {
		t.terminal = t.eventType
	}
	return t.usage
}

func (t *streamUsageTracker) consumeLine(line string) {
	line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
	if line == "" {
		if t.terminal == "" && isResponsesTerminalEvent(t.eventType) {
			t.terminal = t.eventType
		}
		t.eventType = ""
		return
	}
	if strings.HasPrefix(line, "event:") {
		t.eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		return
	}
	if !strings.HasPrefix(line, "data:") {
		return
	}
	data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if data == "" || data == "[DONE]" {
		return
	}
	parsed := parseUsage([]byte(data), "application/json")
	if parsed.TotalTokens > 0 || parsed.PromptTokens > 0 || parsed.CompletionTokens > 0 {
		t.usage = parsed
	}
	eventType, eventError, responseSnapshot, sequenceNumber, hasSequenceNumber := parseResponsesStreamEvent([]byte(data))
	if eventType == "" {
		eventType = t.eventType
	}
	if len(responseSnapshot) > 0 {
		t.responseSnapshot = append(t.responseSnapshot[:0], responseSnapshot...)
	}
	if hasSequenceNumber {
		t.sequenceNumber = sequenceNumber
		t.hasSequenceNumber = true
	}
	if isResponsesTerminalEvent(eventType) {
		t.terminal = eventType
		if eventError != "" {
			t.terminalError = eventError
		}
	}
}

func isResponsesTerminalEvent(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case "response.completed", "response.failed", "response.incomplete":
		return true
	default:
		return false
	}
}

func parseResponsesStreamEvent(data []byte) (string, string, json.RawMessage, int64, bool) {
	var payload struct {
		Type           string          `json:"type"`
		SequenceNumber *int64          `json:"sequence_number"`
		Error          json.RawMessage `json:"error"`
		Response       json.RawMessage `json:"response"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", "", nil, 0, false
	}
	detail := responseStreamErrorMessage(payload.Error)
	if detail == "" && len(payload.Response) > 0 {
		var response struct {
			Error             json.RawMessage `json:"error"`
			IncompleteDetails struct {
				Reason string `json:"reason"`
			} `json:"incomplete_details"`
		}
		_ = json.Unmarshal(payload.Response, &response)
		detail = responseStreamErrorMessage(response.Error)
		if detail == "" {
			detail = strings.TrimSpace(response.IncompleteDetails.Reason)
		}
	}
	responseSnapshot := json.RawMessage(nil)
	if len(payload.Response) > 0 && string(payload.Response) != "null" {
		responseSnapshot = append(responseSnapshot, payload.Response...)
	}
	sequenceNumber := int64(0)
	hasSequenceNumber := false
	if payload.SequenceNumber != nil {
		sequenceNumber = *payload.SequenceNumber
		hasSequenceNumber = true
	}
	return strings.TrimSpace(payload.Type), detail, responseSnapshot, sequenceNumber, hasSequenceNumber
}

func responseStreamErrorMessage(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var object struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	}
	if err := json.Unmarshal(raw, &object); err == nil {
		message := strings.TrimSpace(object.Message)
		if message != "" {
			return message
		}
		if code := strings.TrimSpace(object.Code); code != "" {
			return code
		}
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	return ""
}

func calculateQuota(usage vos.Usage) int64 {
	if usage.TotalTokens > 0 {
		return usage.TotalTokens
	}
	return usage.PromptTokens + usage.CompletionTokens
}

func calculateFinalQuota(modelName string, group string, usage vos.Usage, body []byte, reservedQuota int64) int64 {
	quota := calculateQuota(usage)
	if quota > 0 {
		return quota
	}
	if reservedQuota > 0 {
		return reservedQuota
	}
	return estimateQuotaFromBody(body)
}

func estimateQuotaFromBody(body []byte) int64 {
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		if len(body) == 0 {
			return 1
		}
		return int64(len(body)/4 + 1)
	}
	b, _ := json.Marshal(payload)
	if len(b) == 0 {
		return 1
	}
	return int64(len(b)/4 + 1)
}

func estimatePreauthorizeUsage(endpoint RelayEndpoint, body []byte) vos.Usage {
	promptTokens := estimateQuotaFromBody(body)
	completionTokens := int64(0)
	if shouldReserveOutputTokens(endpoint) {
		completionTokens = extractMaxOutputTokens(body)
		if completionTokens <= 0 {
			completionTokens = OptionServiceApp.Int64("relay.billing_default_output_tokens", 4096)
		}
		if completionTokens < 0 {
			completionTokens = 0
		}
	}
	return vos.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
	}
}

func shouldReserveOutputTokens(endpoint RelayEndpoint) bool {
	path := strings.ToLower(endpoint.UpstreamPath)
	if strings.Contains(path, "/embeddings") ||
		strings.Contains(path, "/moderations") ||
		strings.Contains(path, "/rerank") ||
		strings.Contains(path, "/images/") ||
		strings.Contains(path, "/audio/") {
		return false
	}
	return true
}

func extractMaxOutputTokens(body []byte) int64 {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0
	}
	if value := firstInt64Value(payload, "max_tokens", "max_completion_tokens", "max_output_tokens"); value > 0 {
		return value
	}
	for _, key := range []string{"text", "reasoning", "thinking", "extra_body", "extraBody"} {
		nested, ok := payload[key].(map[string]any)
		if !ok {
			continue
		}
		if value := firstInt64Value(nested, "max_tokens", "max_completion_tokens", "max_output_tokens", "budget_tokens"); value > 0 {
			return value
		}
	}
	return 0
}

func firstInt64Value(payload map[string]any, keys ...string) int64 {
	for _, key := range keys {
		if value, ok := payload[key]; ok {
			if parsed := int64FromJSONValue(value); parsed > 0 {
				return parsed
			}
		}
	}
	return 0
}

func int64FromJSONValue(value any) int64 {
	switch item := value.(type) {
	case float64:
		return int64(item)
	case int:
		return int64(item)
	case int64:
		return item
	case json.Number:
		parsed, _ := item.Int64()
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(item), 10, 64)
		return parsed
	default:
		return 0
	}
}

func isStreamRequest(body []byte) bool {
	var req vos.ModelRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}
	return req.Stream
}

func extractUpstreamRequestID(result *RelayResult) string {
	if result == nil {
		return ""
	}
	for _, header := range []string{"X-Request-Id", "X-Upstream-Request-Id", "Request-Id", "Apim-Request-Id"} {
		if value := strings.TrimSpace(result.Header.Get(header)); value != "" {
			return value
		}
	}
	return ""
}

func firstResponseTime(result *RelayResult) int64 {
	if result == nil || result.FirstResponseTimeMs < 0 {
		return 0
	}
	return result.FirstResponseTimeMs
}

func usageLogTiming(result *RelayResult, body []byte, attempts int) UpstreamTiming {
	timing := UpstreamTiming{
		RequestBodyBytes: int64(len(body)),
		AttemptCount:     attempts,
	}
	if result != nil {
		timing = result.Timing
		if timing.RequestBodyBytes <= 0 {
			timing.RequestBodyBytes = int64(len(body))
		}
		if timing.AttemptCount <= 0 {
			timing.AttemptCount = attempts
		}
	}
	return timing
}

func buildUsageLog(c *gin.Context, token *domains.ApiToken, provider *domains.VendorMeta, modelName string, usage vos.Usage, quota int64, useTimeMs int64, firstResponseTimeMs int64, timing UpstreamTiming, stream bool, status string, content string, body []byte, upstreamRequestID string, routeAttempts []RelayAttempt, relayResults ...*RelayResult) *domains.UsageLog {
	if len(content) > 2000 {
		content = content[:2000]
	}
	providerGuid := ""
	providerName := ""
	if provider != nil {
		providerGuid = provider.Guid
		providerName = provider.DisplayName
		if providerName == "" {
			providerName = provider.VendorName
		}
	}
	detail := PricingServiceApp.CalculateQuotaDetail(modelName, token.Group, usage, estimateQuotaFromBody(body))
	detail.Quota = quota
	detail = usageLogBillingDetail(status, detail)
	return &domains.UsageLog{
		UserGuid:             token.UserGuid,
		TokenGuid:            token.Guid,
		TokenName:            token.Name,
		ProviderGuid:         providerGuid,
		ProviderName:         providerName,
		ModelName:            modelName,
		Quota:                quota,
		Cost:                 detail.FinalCost,
		PromptTokens:         usage.PromptTokens,
		CompletionTokens:     usage.CompletionTokens,
		UseTimeMs:            useTimeMs,
		FirstResponseTimeMs:  firstResponseTimeMs,
		RequestBodyBytes:     timing.RequestBodyBytes,
		DNSLookupTimeMs:      timing.DNSLookupTimeMs,
		ConnectTimeMs:        timing.ConnectTimeMs,
		TLSHandshakeTimeMs:   timing.TLSHandshakeTimeMs,
		RequestWriteTimeMs:   timing.RequestWriteTimeMs,
		ResponseHeaderTimeMs: timing.ResponseHeaderTimeMs,
		UpstreamTotalTimeMs:  timing.UpstreamTotalTimeMs,
		ConnectionReused:     timing.ConnectionReused,
		AttemptCount:         timing.AttemptCount,
		IsStream:             stream,
		Status:               status,
		Content:              content,
		RequestID:            c.GetHeader("X-Request-Id"),
		UpstreamRequestID:    upstreamRequestID,
		ClientIP:             c.ClientIP(),
		Source:               domains.UsageLogSourceUser,
		Other:                buildUsageLogOther(token, body, detail, routeAttempts, relayResults...),
	}
}

func usageLogBillingDetail(status string, detail QuotaCalculationDetail) QuotaCalculationDetail {
	if status != "success" && status != "ok" {
		detail.FinalCost = 0
	}
	return detail
}

func buildUsageLogOther(token *domains.ApiToken, body []byte, detail QuotaCalculationDetail, routeAttempts []RelayAttempt, relayResults ...*RelayResult) string {
	group := normalizeGroup(token.Group)
	values := map[string]any{
		"group":               group,
		"cachedTokens":        detail.CachedTokens,
		"billingMode":         detail.BillingMode,
		"pricingMatched":      detail.PricingMatched,
		"promptMultiplier":    detail.PromptMultiplier,
		"outputMultiplier":    detail.OutputMultiplier,
		"cacheMultiplier":     detail.CacheMultiplier,
		"quotaMultiplier":     detail.QuotaMultiplier,
		"groupMultiplier":     detail.GroupMultiplier,
		"officialPricing":     detail.OfficialPricing,
		"regularPromptTokens": detail.RegularPromptTokens,
		"completionTokens":    detail.CompletionTokens,
		"fallbackQuota":       detail.FallbackQuota,
		"quota":               detail.Quota,
		"amountMicros":        CostToAmountMicros(detail.FinalCost),
	}
	if detail.OfficialPricing {
		values["officialProvider"] = detail.OfficialProvider
		values["officialPriceUnit"] = detail.OfficialPriceUnit
		values["officialInputPrice"] = detail.OfficialInputPrice
		values["officialOutputPrice"] = detail.OfficialOutputPrice
		values["officialCachePrice"] = detail.OfficialCachePrice
		values["priceUnitTokens"] = detail.PriceUnitTokens
		values["rawCost"] = detail.RawCost
		values["finalCost"] = detail.FinalCost
	}
	if detail.PricingModel != "" {
		values["pricingModel"] = detail.PricingModel
	}
	if detail.PricingGroup != "" {
		values["pricingGroup"] = detail.PricingGroup
	}
	if reasoningEffort := extractReasoningEffort(body); reasoningEffort != "" {
		values["reasoningEffort"] = reasoningEffort
	}
	if len(routeAttempts) > 0 {
		values["relayAttempts"] = routeAttempts
		values["retryCount"] = relayRetryCount(routeAttempts)
		providerFallback := relayProviderFallback(routeAttempts)
		values["providerFallback"] = providerFallback
		values["modelDowngraded"] = false
		values["upstreamModelChanged"] = relayUpstreamModelChanged(routeAttempts)
		values["routingType"] = "provider"
		if providerFallback {
			values["routingType"] = "provider_fallback"
		}
		if finalAttempt, ok := finalRelayAttempt(routeAttempts); ok {
			values["finalProviderGuid"] = finalAttempt.ProviderGuid
			values["finalProviderName"] = finalAttempt.ProviderName
			values["finalUpstreamModel"] = finalAttempt.UpstreamModel
			values["modelMapped"] = finalAttempt.UpstreamModel != "" && finalAttempt.UpstreamModel != finalAttempt.RequestedModel
		}
	}
	if len(relayResults) > 0 && relayResults[0] != nil {
		result := relayResults[0]
		if result.StreamTerminal != "" {
			values["streamTerminal"] = result.StreamTerminal
		}
		if result.StreamTerminalError != "" {
			values["streamTerminalError"] = result.StreamTerminalError
		}
		if result.StreamSynthesized {
			values["streamSynthesized"] = true
			values["synthesizedReason"] = synthesizedStreamLogContent(result)
		}
	}
	data, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return string(data)
}

func relayRetryCount(attempts []RelayAttempt) int {
	count := 0
	for _, attempt := range attempts {
		if attempt.Attempt > 0 {
			count++
		}
	}
	if count <= 1 {
		return 0
	}
	return count - 1
}

func relayProviderFallback(attempts []RelayAttempt) bool {
	providers := map[string]struct{}{}
	for _, attempt := range attempts {
		if attempt.Attempt <= 0 || attempt.ProviderGuid == "" {
			continue
		}
		providers[attempt.ProviderGuid] = struct{}{}
	}
	return len(providers) > 1
}

func relayUpstreamModelChanged(attempts []RelayAttempt) bool {
	models := map[string]struct{}{}
	for _, attempt := range attempts {
		if attempt.Attempt <= 0 {
			continue
		}
		model := strings.TrimSpace(attempt.UpstreamModel)
		if model != "" {
			models[model] = struct{}{}
		}
	}
	return len(models) > 1
}

func finalRelayAttempt(attempts []RelayAttempt) (RelayAttempt, bool) {
	for i := len(attempts) - 1; i >= 0; i-- {
		if attempts[i].Attempt > 0 {
			return attempts[i], true
		}
	}
	return RelayAttempt{}, false
}

func marshalBillingMeta(detail QuotaCalculationDetail, amountMicros int64) string {
	values := map[string]any{
		"billingMode":      detail.BillingMode,
		"pricingMatched":   detail.PricingMatched,
		"officialPricing":  detail.OfficialPricing,
		"rawCost":          detail.RawCost,
		"finalCost":        detail.FinalCost,
		"amountMicros":     amountMicros,
		"groupMultiplier":  detail.GroupMultiplier,
		"promptTokens":     detail.RegularPromptTokens + detail.CachedTokens,
		"cachedTokens":     detail.CachedTokens,
		"completionTokens": detail.CompletionTokens,
	}
	if detail.PricingModel != "" {
		values["pricingModel"] = detail.PricingModel
	}
	if detail.PricingGroup != "" {
		values["pricingGroup"] = detail.PricingGroup
	}
	if detail.OfficialProvider != "" {
		values["officialProvider"] = detail.OfficialProvider
	}
	if detail.OfficialPriceUnit != "" {
		values["officialPriceUnit"] = detail.OfficialPriceUnit
	}
	data, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return string(data)
}

func extractReasoningEffort(body []byte) string {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	if value := firstStringValue(payload, "reasoning_effort", "reasoningEffort"); value != "" {
		return value
	}
	for _, key := range []string{"extra_body", "extraBody", "metadata"} {
		if nested, ok := payload[key].(map[string]any); ok {
			if value := firstStringValue(nested, "reasoning_effort", "reasoningEffort"); value != "" {
				return value
			}
		}
	}
	for _, key := range []string{"reasoning", "thinking"} {
		nested, ok := payload[key].(map[string]any)
		if !ok {
			continue
		}
		if value := firstStringValue(nested, "effort", "reasoning_effort", "reasoningEffort"); value != "" {
			return value
		}
		if value := firstStringValue(nested, "budget_tokens", "max_tokens"); value != "" {
			return "预算 " + value + " tokens"
		}
	}
	return ""
}

func firstStringValue(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := payload[key]; ok {
			if text := stringifyLogValue(value); text != "" {
				return text
			}
		}
	}
	return ""
}

func stringifyLogValue(value any) string {
	switch item := value.(type) {
	case string:
		return strings.TrimSpace(item)
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.6f", item), "0"), ".")
	case int:
		return fmt.Sprintf("%d", item)
	case int64:
		return fmt.Sprintf("%d", item)
	case json.Number:
		return item.String()
	default:
		return ""
	}
}
