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
	"net/textproto"
	"net/url"
	"regexp"
	"strings"
	"time"

	"navapi-go/constants"
	"navapi-go/domains"
	"navapi-go/dto"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/global"
	"gorm.io/gorm"
)

type RelayService struct {
	client *http.Client
}

var RelayServiceApp = RelayService{
	client: &http.Client{Timeout: 10 * time.Minute},
}

type RelayEndpoint struct {
	UpstreamPath  string
	Method        string
	Format        string
	ModelFromPath bool
	NoBilling     bool
	DefaultModel  string
}

type RelayResult struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	Usage      dto.Usage
}

type preparedRelay struct {
	Body          []byte
	ModelName     string
	Candidates    []domains.Channel
	IsStream      bool
	ReservedQuota int64
}

type RelayHTTPError struct {
	StatusCode int
	Message    string
}

func (e *RelayHTTPError) Error() string {
	return e.Message
}

// RelayHTTP is the single entry point used by handlers. It prepares the request
// once, reserves quota before touching upstream, then chooses buffered or live
// streaming delivery based on the original client payload.
func (s RelayService) RelayHTTP(c *gin.Context, token *domains.ApiToken, endpoint RelayEndpoint) (*RelayResult, bool, error) {
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
	if err := checkModelRateLimit(token, modelName); err != nil {
		return nil, err
	}
	if err := TokenServiceApp.CheckModel(token, modelName); err != nil {
		return nil, err
	}
	candidates, err := ChannelServiceApp.FindCandidatesForModelAndType(modelName, token.Group, endpoint.Format)
	if err != nil {
		return nil, fmt.Errorf("no available channel for model %s", modelName)
	}
	candidates = ChannelServiceApp.ApplyAffinity(token.Guid, modelName, candidates)
	reservedQuota := int64(0)
	if !endpoint.NoBilling {
		reservedQuota = PricingServiceApp.CalculateQuota(modelName, token.Group, dto.Usage{}, estimateQuotaFromBody(body))
		if err := s.reserveQuota(token, reservedQuota); err != nil {
			return nil, err
		}
	}
	return &preparedRelay{
		Body:          body,
		ModelName:     modelName,
		Candidates:    candidates,
		IsStream:      isStreamRequest(body),
		ReservedQuota: reservedQuota,
	}, nil
}

func (s RelayService) relayBuffered(c *gin.Context, token *domains.ApiToken, endpoint RelayEndpoint, prepared *preparedRelay) (*RelayResult, error) {
	start := time.Now()
	var channel *domains.Channel
	var result *RelayResult
	var err error
	for i := range prepared.Candidates {
		current := prepared.Candidates[i]
		forwardBody, upstreamPath := buildUpstreamRequest(&current, prepared.ModelName, endpoint, prepared.Body, c.GetHeader("Content-Type"))
		channel = &current
		result, err = s.forward(c.Request.Context(), &current, endpoint.Method, upstreamPath, forwardBody, c.Request.Header, c.Request.URL.RawQuery)
		if err != nil && i < len(prepared.Candidates)-1 {
			continue
		}
		if err == nil && result != nil && shouldRetryRelayStatus(result.StatusCode) && i < len(prepared.Candidates)-1 {
			maybeAutoDisableChannel(&current, result)
			continue
		}
		break
	}
	useTime := time.Since(start).Milliseconds()
	status := "success"
	content := ""
	if err != nil {
		status = "error"
		content = err.Error()
		_ = s.refundReservedQuota(token, prepared.ReservedQuota)
		_ = LogServiceApp.Create(buildUsageLog(c, token, channel, prepared.ModelName, dto.Usage{}, 0, useTime, prepared.IsStream, status, content))
		return nil, err
	}
	if result.StatusCode >= http.StatusBadRequest {
		status = "error"
		content = string(result.Body)
		maybeAutoDisableChannel(channel, result)
		_ = s.refundReservedQuota(token, prepared.ReservedQuota)
		_ = LogServiceApp.Create(buildUsageLog(c, token, channel, prepared.ModelName, result.Usage, 0, useTime, prepared.IsStream, status, content))
		return result, nil
	}
	if channel != nil {
		ChannelServiceApp.RememberAffinity(token.Guid, prepared.ModelName, channel.Id)
	}
	quota := calculateFinalQuota(prepared.ModelName, token.Group, result.Usage, prepared.Body, prepared.ReservedQuota)
	if !endpoint.NoBilling {
		if err := s.settleReservedQuota(token, channel, prepared.ReservedQuota, quota); err != nil {
			status = "error"
			content = err.Error()
			_ = LogServiceApp.Create(buildUsageLog(c, token, channel, prepared.ModelName, result.Usage, 0, useTime, prepared.IsStream, status, content))
			return nil, err
		}
	} else {
		quota = 0
	}
	_ = LogServiceApp.Create(buildUsageLog(c, token, channel, prepared.ModelName, result.Usage, quota, useTime, prepared.IsStream, status, content))
	return result, nil
}

func (s RelayService) relayStream(c *gin.Context, token *domains.ApiToken, endpoint RelayEndpoint, prepared *preparedRelay) error {
	start := time.Now()
	var channel *domains.Channel
	var result *RelayResult
	var err error
	for i := range prepared.Candidates {
		current := prepared.Candidates[i]
		forwardBody, upstreamPath := buildUpstreamRequest(&current, prepared.ModelName, endpoint, prepared.Body, c.GetHeader("Content-Type"))
		channel = &current
		result, err = s.forwardStream(c, &current, endpoint.Method, upstreamPath, forwardBody, c.Request.Header, c.Request.URL.RawQuery, i < len(prepared.Candidates)-1)
		if err != nil && i < len(prepared.Candidates)-1 {
			continue
		}
		if err == nil && result != nil && shouldRetryRelayStatus(result.StatusCode) && i < len(prepared.Candidates)-1 {
			maybeAutoDisableChannel(&current, result)
			continue
		}
		break
	}

	useTime := time.Since(start).Milliseconds()
	if err != nil {
		_ = s.refundReservedQuota(token, prepared.ReservedQuota)
		_ = LogServiceApp.Create(buildUsageLog(c, token, channel, prepared.ModelName, dto.Usage{}, 0, useTime, prepared.IsStream, "error", err.Error()))
		return err
	}
	if result == nil {
		err = errors.New("upstream response is empty")
		_ = s.refundReservedQuota(token, prepared.ReservedQuota)
		_ = LogServiceApp.Create(buildUsageLog(c, token, channel, prepared.ModelName, dto.Usage{}, 0, useTime, prepared.IsStream, "error", err.Error()))
		return err
	}
	if result.StatusCode >= http.StatusBadRequest {
		maybeAutoDisableChannel(channel, result)
		_ = s.refundReservedQuota(token, prepared.ReservedQuota)
		_ = LogServiceApp.Create(buildUsageLog(c, token, channel, prepared.ModelName, result.Usage, 0, useTime, prepared.IsStream, "error", string(result.Body)))
		return nil
	}
	if channel != nil {
		ChannelServiceApp.RememberAffinity(token.Guid, prepared.ModelName, channel.Id)
	}
	quota := calculateFinalQuota(prepared.ModelName, token.Group, result.Usage, prepared.Body, prepared.ReservedQuota)
	if endpoint.NoBilling {
		quota = 0
	} else if err := s.settleReservedQuota(token, channel, prepared.ReservedQuota, quota); err != nil {
		// The stream may already be on the wire, so settlement failures are
		// recorded in logs instead of trying to replace the response body.
		_ = LogServiceApp.Create(buildUsageLog(c, token, channel, prepared.ModelName, result.Usage, 0, useTime, prepared.IsStream, "error", err.Error()))
		return nil
	}
	_ = LogServiceApp.Create(buildUsageLog(c, token, channel, prepared.ModelName, result.Usage, quota, useTime, prepared.IsStream, "success", ""))
	return nil
}

func checkModelRateLimit(token *domains.ApiToken, modelName string) error {
	limit := OptionServiceApp.Int64("relay.model_rate_limit_count", 0)
	windowSeconds := OptionServiceApp.Int64("relay.model_rate_limit_window_seconds", 60)
	if token == nil || limit <= 0 || windowSeconds <= 0 {
		return nil
	}
	key := token.Guid + ":" + strings.TrimSpace(modelName)
	ok, retryAfter := RateLimitServiceApp.Allow(key, limit, time.Duration(windowSeconds)*time.Second)
	if ok {
		return nil
	}
	message := "rate limit exceeded"
	if retryAfter > 0 {
		message = fmt.Sprintf("rate limit exceeded, retry after %s", retryAfter.Round(time.Second))
	}
	return &RelayHTTPError{StatusCode: http.StatusTooManyRequests, Message: message}
}

func shouldRetryRelayStatus(statusCode int) bool {
	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden || statusCode == http.StatusRequestTimeout || statusCode == http.StatusConflict || statusCode == http.StatusTooManyRequests {
		return true
	}
	return statusCode >= http.StatusInternalServerError
}

func maybeAutoDisableChannel(channel *domains.Channel, result *RelayResult) {
	if channel == nil || result == nil {
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
	_ = ChannelServiceApp.AutoDisable(channel.Id, reason)
}

func buildUpstreamRequest(channel *domains.Channel, modelName string, endpoint RelayEndpoint, body []byte, contentType string) ([]byte, string) {
	upstreamModel := ChannelServiceApp.MapModel(channel, modelName)
	if endpoint.ModelFromPath {
		return body, rewriteModelInPath(endpoint.UpstreamPath, upstreamModel)
	}
	return rewriteBodyModel(body, upstreamModel, contentType), endpoint.UpstreamPath
}

func (s RelayService) forward(ctx context.Context, channel *domains.Channel, method string, upstreamPath string, body []byte, incoming http.Header, rawQuery string) (*RelayResult, error) {
	baseURL := strings.TrimRight(channel.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL(channel.Type)
	}
	targetURL := baseURL + upstreamPath
	if rawQuery != "" && channel.Type != constants.ChannelTypeGemini {
		targetURL += "?" + rawQuery
	}
	if channel.Type == constants.ChannelTypeGemini {
		targetURL = attachGeminiKey(targetURL, channel.Key, rawQuery)
	}
	targetURL = applyParamOverride(targetURL, channel.ParamOverride)
	req, err := http.NewRequestWithContext(ctx, method, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	copyForwardHeaders(req.Header, incoming)
	setupAuthHeaders(req.Header, channel)
	applyHeaderOverride(req.Header, channel.HeaderOverride)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &RelayResult{
		StatusCode: resp.StatusCode,
		Header:     resp.Header.Clone(),
		Body:       respBody,
		Usage:      parseUsage(respBody, resp.Header.Get("Content-Type")),
	}, nil
}

func (s RelayService) forwardStream(c *gin.Context, channel *domains.Channel, method string, upstreamPath string, body []byte, incoming http.Header, rawQuery string, canRetry bool) (*RelayResult, error) {
	baseURL := strings.TrimRight(channel.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL(channel.Type)
	}
	targetURL := baseURL + upstreamPath
	if rawQuery != "" && channel.Type != constants.ChannelTypeGemini {
		targetURL += "?" + rawQuery
	}
	if channel.Type == constants.ChannelTypeGemini {
		targetURL = attachGeminiKey(targetURL, channel.Key, rawQuery)
	}
	targetURL = applyParamOverride(targetURL, channel.ParamOverride)
	req, err := http.NewRequestWithContext(c.Request.Context(), method, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	copyForwardHeaders(req.Header, incoming)
	setupAuthHeaders(req.Header, channel)
	applyHeaderOverride(req.Header, channel.HeaderOverride)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest || (canRetry && shouldRetryRelayStatus(resp.StatusCode)) {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, readErr
		}
		if !canRetry {
			copyResponseHeaders(c.Writer.Header(), resp.Header)
			c.Data(resp.StatusCode, contentTypeOrJSON(resp.Header), respBody)
		}
		return &RelayResult{
			StatusCode: resp.StatusCode,
			Header:     resp.Header.Clone(),
			Body:       respBody,
			Usage:      parseUsage(respBody, resp.Header.Get("Content-Type")),
		}, nil
	}

	copyResponseHeaders(c.Writer.Header(), resp.Header)
	c.Status(resp.StatusCode)
	tracker := &streamUsageTracker{}
	buf := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			tracker.Write(chunk)
			if _, writeErr := c.Writer.Write(chunk); writeErr != nil {
				return nil, writeErr
			}
			c.Writer.Flush()
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
	}
	return &RelayResult{StatusCode: resp.StatusCode, Header: resp.Header.Clone(), Usage: tracker.Finish()}, nil
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

func (s RelayService) reserveQuota(token *domains.ApiToken, quota int64) error {
	if quota <= 0 {
		return nil
	}
	return global.NAV_DB.Transaction(func(tx *gorm.DB) error {
		if err := TokenServiceApp.Consume(tx, token.Id, quota); err != nil {
			return err
		}
		return UserQuotaServiceApp.Consume(tx, token.UserGuid, quota)
	})
}

func (s RelayService) refundReservedQuota(token *domains.ApiToken, quota int64) error {
	if quota <= 0 {
		return nil
	}
	return global.NAV_DB.Transaction(func(tx *gorm.DB) error {
		if err := TokenServiceApp.Refund(tx, token.Id, quota); err != nil {
			return err
		}
		return UserQuotaServiceApp.Refund(tx, token.UserGuid, quota)
	})
}

// settleReservedQuota converts the reservation into the final charge. Only the
// delta touches token/user counters because reserveQuota already moved them.
func (s RelayService) settleReservedQuota(token *domains.ApiToken, channel *domains.Channel, reservedQuota int64, finalQuota int64) error {
	return global.NAV_DB.Transaction(func(tx *gorm.DB) error {
		delta := finalQuota - reservedQuota
		switch {
		case delta > 0:
			if err := TokenServiceApp.Consume(tx, token.Id, delta); err != nil {
				return err
			}
			if err := UserQuotaServiceApp.Consume(tx, token.UserGuid, delta); err != nil {
				return err
			}
		case delta < 0:
			refund := -delta
			if err := TokenServiceApp.Refund(tx, token.Id, refund); err != nil {
				return err
			}
			if err := UserQuotaServiceApp.Refund(tx, token.UserGuid, refund); err != nil {
				return err
			}
		}
		if channel == nil || finalQuota <= 0 {
			return nil
		}
		return tx.Model(&domains.Channel{}).Where("id = ?", channel.Id).
			UpdateColumn("used_quota", gorm.Expr("used_quota + ?", finalQuota)).Error
	})
}

func defaultBaseURL(channelType string) string {
	switch channelType {
	case constants.ChannelTypeAnthropic:
		return "https://api.anthropic.com"
	case constants.ChannelTypeGemini:
		return "https://generativelanguage.googleapis.com"
	default:
		return "https://api.openai.com"
	}
}

func setupAuthHeaders(header http.Header, channel *domains.Channel) {
	key := strings.TrimSpace(ChannelServiceApp.NextKey(channel))
	switch channel.Type {
	case constants.ChannelTypeAnthropic:
		header.Set("x-api-key", key)
		if header.Get("anthropic-version") == "" {
			header.Set("anthropic-version", "2023-06-01")
		}
		header.Del("Authorization")
	case constants.ChannelTypeGemini:
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
	var req dto.ModelRequest
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
	if model == "" || !strings.Contains(contentType, "application/json") {
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

func parseUsage(body []byte, contentType string) dto.Usage {
	if strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		if usage := parseStreamUsage(body); usage.TotalTokens > 0 || usage.PromptTokens > 0 || usage.CompletionTokens > 0 {
			return usage
		}
	}
	var payload struct {
		Usage dto.Usage `json:"usage"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return dto.Usage{}
	}
	return normalizeUsage(payload.Usage)
}

func normalizeUsage(usage dto.Usage) dto.Usage {
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

func parseStreamUsage(body []byte) dto.Usage {
	var usage dto.Usage
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
	pending string
	usage   dto.Usage
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

func (t *streamUsageTracker) Finish() dto.Usage {
	if strings.TrimSpace(t.pending) != "" {
		t.consumeLine(t.pending)
	}
	return t.usage
}

func (t *streamUsageTracker) consumeLine(line string) {
	line = strings.TrimSpace(line)
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
}

func calculateQuota(usage dto.Usage) int64 {
	if usage.TotalTokens > 0 {
		return usage.TotalTokens
	}
	return usage.PromptTokens + usage.CompletionTokens
}

func calculateFinalQuota(modelName string, group string, usage dto.Usage, body []byte, reservedQuota int64) int64 {
	quota := calculateQuota(usage)
	if quota > 0 {
		return PricingServiceApp.CalculateQuota(modelName, group, usage, quota)
	}
	if reservedQuota > 0 {
		return reservedQuota
	}
	return PricingServiceApp.CalculateQuota(modelName, group, usage, estimateQuotaFromBody(body))
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

func isStreamRequest(body []byte) bool {
	var req dto.ModelRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}
	return req.Stream
}

func buildUsageLog(c *gin.Context, token *domains.ApiToken, channel *domains.Channel, modelName string, usage dto.Usage, quota int64, useTimeMs int64, stream bool, status string, content string) *domains.UsageLog {
	if len(content) > 2000 {
		content = content[:2000]
	}
	channelGuid := ""
	channelName := ""
	if channel != nil {
		channelGuid = channel.Guid
		channelName = channel.Name
	}
	return &domains.UsageLog{
		UserGuid:          token.UserGuid,
		TokenGuid:         token.Guid,
		TokenName:         token.Name,
		ChannelGuid:       channelGuid,
		ChannelName:       channelName,
		ModelName:         modelName,
		Quota:             quota,
		PromptTokens:      usage.PromptTokens,
		CompletionTokens:  usage.CompletionTokens,
		UseTimeMs:         useTimeMs,
		IsStream:          stream,
		Status:            status,
		Content:           content,
		RequestID:         c.GetHeader("X-Request-Id"),
		UpstreamRequestID: c.GetHeader("X-Upstream-Request-Id"),
		ClientIP:          c.ClientIP(),
	}
}
