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
	client       *http.Client
	streamClient *http.Client
}

var RelayServiceApp = new(RelayService{
	client:       &http.Client{Timeout: 10 * time.Minute, Transport: cloneDefaultTransport()},
	streamClient: newStreamHTTPClient(),
})

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
	release, err := UserConcurrencyServiceApp.Acquire(token.UserGuid)
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
	var circuitRetryAfter time.Duration
	for i := range prepared.Candidates {
		current := prepared.Candidates[i]
		permit, retryAfter, available := ProviderCircuitBreakerApp.TryAcquire(current.Guid, prepared.ModelName, endpoint.UpstreamPath)
		if !available {
			if retryAfter > 0 && (circuitRetryAfter <= 0 || retryAfter < circuitRetryAfter) {
				circuitRetryAfter = retryAfter
			}
			continue
		}
		attempts++
		forwardBody, upstreamPath := buildUpstreamRequest(&current, prepared.ModelName, endpoint, prepared.Body, c.GetHeader("Content-Type"))
		provider = &current
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
		if err != nil && i < len(prepared.Candidates)-1 {
			continue
		}
		if err == nil && result != nil && shouldRetryRelayStatus(result.StatusCode) && i < len(prepared.Candidates)-1 {
			continue
		}
		break
	}
	useTime := time.Since(start).Milliseconds()
	if attempts == 0 {
		err = providerCircuitUnavailableError(circuitRetryAfter)
		s.cancelReservation(token, prepared.Reservation, err.Error())
		return nil, err
	}
	status := "success"
	content := ""
	if err != nil {
		status = "error"
		content = err.Error()
		s.cancelReservation(token, prepared.Reservation, content)
		_ = LogServiceApp.Create(buildUsageLog(c, token, provider, prepared.ModelName, vos.Usage{}, 0, useTime, 0, usageLogTiming(result, prepared.Body, attempts), prepared.IsStream, status, content, prepared.Body, ""))
		return nil, err
	}
	if result.StatusCode >= http.StatusBadRequest {
		status = "error"
		content = string(result.Body)
		s.cancelReservation(token, prepared.Reservation, content)
		_ = LogServiceApp.Create(buildUsageLog(c, token, provider, prepared.ModelName, result.Usage, 0, useTime, firstResponseTime(result), usageLogTiming(result, prepared.Body, attempts), prepared.IsStream, status, content, prepared.Body, extractUpstreamRequestID(result)))
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
			_ = LogServiceApp.Create(buildUsageLog(c, token, provider, prepared.ModelName, result.Usage, 0, useTime, firstResponseTime(result), usageLogTiming(result, prepared.Body, attempts), prepared.IsStream, status, content, prepared.Body, extractUpstreamRequestID(result)))
			return nil, err
		}
	} else {
		quota = 0
	}
	_ = LogServiceApp.Create(buildUsageLog(c, token, provider, prepared.ModelName, result.Usage, quota, useTime, firstResponseTime(result), usageLogTiming(result, prepared.Body, attempts), prepared.IsStream, status, content, prepared.Body, extractUpstreamRequestID(result)))
	return result, nil
}

func (s RelayService) relayStream(c *gin.Context, token *domains.ApiToken, endpoint RelayEndpoint, prepared *preparedRelay) error {
	start := time.Now()
	var provider *domains.VendorMeta
	var result *RelayResult
	var err error
	attempts := 0
	var circuitRetryAfter time.Duration
	for i := range prepared.Candidates {
		current := prepared.Candidates[i]
		permit, retryAfter, available := ProviderCircuitBreakerApp.TryAcquire(current.Guid, prepared.ModelName, endpoint.UpstreamPath)
		if !available {
			if retryAfter > 0 && (circuitRetryAfter <= 0 || retryAfter < circuitRetryAfter) {
				circuitRetryAfter = retryAfter
			}
			continue
		}
		attempts++
		forwardBody, upstreamPath := buildUpstreamRequest(&current, prepared.ModelName, endpoint, prepared.Body, c.GetHeader("Content-Type"))
		provider = &current
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
		if err != nil && i < len(prepared.Candidates)-1 && canRetryStreamAttempt(result) {
			continue
		}
		if err == nil && result != nil && shouldRetryRelayStatus(result.StatusCode) && i < len(prepared.Candidates)-1 {
			continue
		}
		break
	}

	useTime := time.Since(start).Milliseconds()
	if attempts == 0 {
		err = providerCircuitUnavailableError(circuitRetryAfter)
		s.cancelReservation(token, prepared.Reservation, err.Error())
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
		_ = LogServiceApp.Create(buildUsageLog(c, token, provider, prepared.ModelName, usage, 0, useTime, firstResponseMs, usageLogTiming(result, prepared.Body, attempts), prepared.IsStream, "error", err.Error(), prepared.Body, upstreamRequestID))
		return err
	}
	if result == nil {
		err = errors.New("upstream response is empty")
		s.cancelReservation(token, prepared.Reservation, err.Error())
		_ = LogServiceApp.Create(buildUsageLog(c, token, provider, prepared.ModelName, vos.Usage{}, 0, useTime, 0, usageLogTiming(result, prepared.Body, attempts), prepared.IsStream, "error", err.Error(), prepared.Body, ""))
		return err
	}
	if result.StatusCode >= http.StatusBadRequest {
		if !result.StreamStarted {
			writeBufferedStreamResult(c, result)
		}
		s.cancelReservation(token, prepared.Reservation, string(result.Body))
		_ = LogServiceApp.Create(buildUsageLog(c, token, provider, prepared.ModelName, result.Usage, 0, useTime, firstResponseTime(result), usageLogTiming(result, prepared.Body, attempts), prepared.IsStream, "error", string(result.Body), prepared.Body, extractUpstreamRequestID(result)))
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
			_ = LogServiceApp.Create(buildUsageLog(c, token, provider, prepared.ModelName, result.Usage, 0, useTime, firstResponseTime(result), usageLogTiming(result, prepared.Body, attempts), prepared.IsStream, "error", err.Error(), prepared.Body, extractUpstreamRequestID(result)))
			return nil
		}
	}
	_ = LogServiceApp.Create(buildUsageLog(c, token, provider, prepared.ModelName, result.Usage, quota, useTime, firstResponseTime(result), usageLogTiming(result, prepared.Body, attempts), prepared.IsStream, "success", "", prepared.Body, extractUpstreamRequestID(result)))
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
	targetURL := baseURL + upstreamPath
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
	targetURL := baseURL + upstreamPath
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
	synthesizeResponsesCompleted := requireResponsesTerminal && responsesSynthesizeCompletedOnEOFEnabled()
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
	streamedResponseBytes := int64(0)
	buf := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if responseLimit > 0 && int64(n) > responseLimit-streamedResponseBytes {
				return finishStreamRelayResult(resp, tracker, firstResponseTimeMs, headerResponseTimeMs, requestStart, trace, streamStarted), upstreamResponseTooLargeError(responseLimit)
			}
			streamedResponseBytes += int64(n)
			if !streamStarted {
				copyResponseHeaders(c.Writer.Header(), resp.Header)
				c.Writer.Header().Set("Cache-Control", "no-cache")
				c.Writer.Header().Set("X-Accel-Buffering", "no")
				c.Status(resp.StatusCode)
				streamStarted = true
			}
			if firstResponseTimeMs <= 0 {
				firstResponseTimeMs = time.Since(requestStart).Milliseconds()
			}
			chunk := buf[:n]
			tracker.Write(chunk)
			if _, writeErr := c.Writer.Write(chunk); writeErr != nil {
				return finishStreamRelayResult(resp, tracker, firstResponseTimeMs, headerResponseTimeMs, requestStart, trace, streamStarted), &downstreamStreamWriteError{err: writeErr}
			}
			c.Writer.Flush()
			if requireResponsesTerminal && tracker.terminal != "" {
				break
			}
		}
		if readErr == io.EOF {
			if shouldSynthesizeResponsesCompleted(c, tracker, streamStarted, synthesizeResponsesCompleted) {
				if writeErr := writeSynthesizedResponsesCompletedEvent(c, tracker, "upstream EOF before response.completed"); writeErr != nil {
					return finishStreamRelayResult(resp, tracker, firstResponseTimeMs, headerResponseTimeMs, requestStart, trace, streamStarted), &downstreamStreamWriteError{err: writeErr}
				}
			}
			break
		}
		if readErr != nil {
			if shouldSynthesizeResponsesCompleted(c, tracker, streamStarted, synthesizeResponsesCompleted) {
				if writeErr := writeSynthesizedResponsesCompletedEvent(c, tracker, "upstream stream error before response.completed: "+readErr.Error()); writeErr != nil {
					return finishStreamRelayResult(resp, tracker, firstResponseTimeMs, headerResponseTimeMs, requestStart, trace, streamStarted), &downstreamStreamWriteError{err: writeErr}
				}
				break
			}
			return finishStreamRelayResult(resp, tracker, firstResponseTimeMs, headerResponseTimeMs, requestStart, trace, streamStarted), readErr
		}
	}
	result := finishStreamRelayResult(resp, tracker, firstResponseTimeMs, headerResponseTimeMs, requestStart, trace, streamStarted)
	if !requireResponsesTerminal {
		return result, nil
	}
	return result, responsesStreamTerminalError(result)
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

func responsesSynthesizeCompletedOnEOFEnabled() bool {
	return OptionServiceApp.Bool("relay.responses_synthesize_completed_on_eof", true)
}

func shouldSynthesizeResponsesCompleted(c *gin.Context, tracker *streamUsageTracker, streamStarted bool, enabled bool) bool {
	if !enabled || !streamStarted || tracker == nil || tracker.terminal != "" {
		return false
	}
	if c != nil && c.Request != nil && c.Request.Context().Err() != nil {
		return false
	}
	return true
}

func writeSynthesizedResponsesCompletedEvent(c *gin.Context, tracker *streamUsageTracker, reason string) error {
	if c == nil || tracker == nil {
		return errors.New("stream context is empty")
	}
	chunk := synthesizedResponsesCompletedEvent(tracker, time.Now())
	if _, err := c.Writer.Write(chunk); err != nil {
		return err
	}
	c.Writer.Flush()
	tracker.Write(chunk)
	tracker.terminal = "response.completed"
	tracker.terminalSynthesized = true
	tracker.terminalError = strings.TrimSpace(reason)
	return nil
}

func synthesizedResponsesCompletedEvent(tracker *streamUsageTracker, now time.Time) []byte {
	response := synthesizedResponsesCompletedResponse(tracker, now)
	payload := map[string]any{
		"type":     "response.completed",
		"response": response,
	}
	if tracker != nil && tracker.hasSequenceNumber {
		payload["sequence_number"] = tracker.sequenceNumber + 1
	}
	data, err := json.Marshal(payload)
	if err != nil {
		data = []byte(`{"type":"response.completed","response":{"status":"completed","output":[]}}`)
	}
	return []byte("\n\nevent: response.completed\ndata: " + string(data) + "\n\n")
}

func synthesizedResponsesCompletedResponse(tracker *streamUsageTracker, now time.Time) map[string]any {
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
	response["status"] = "completed"
	if tracker != nil && hasUsageTokens(tracker.usage) {
		response["usage"] = responsesUsagePayload(tracker.usage)
	}
	return response
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
		if _, exists := payload["stream_options"]; !exists {
			return body
		}
		delete(payload, "stream_options")
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

func buildUsageLog(c *gin.Context, token *domains.ApiToken, provider *domains.VendorMeta, modelName string, usage vos.Usage, quota int64, useTimeMs int64, firstResponseTimeMs int64, timing UpstreamTiming, stream bool, status string, content string, body []byte, upstreamRequestID string) *domains.UsageLog {
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
		Other:                buildUsageLogOther(token, body, detail),
	}
}

func buildUsageLogOther(token *domains.ApiToken, body []byte, detail QuotaCalculationDetail) string {
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
	data, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return string(data)
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
