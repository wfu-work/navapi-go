package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"navapi-go/domains"
)

const (
	balanceTemplateOfficial = "official"

	balanceAuthAccessToken = "access_token"
	balanceAuthNone        = "none"

	defaultNewAPIQuotaMultiplier  = 1.0 / 500000.0
	defaultBalanceRefreshInterval = 5 * time.Minute
	defaultBalanceRefreshWorkers  = 4
)

type ProviderBalanceResult struct {
	OK               bool     `json:"ok"`
	Template         string   `json:"template"`
	DetectedTemplate string   `json:"detectedTemplate,omitempty"`
	ResponseTime     int64    `json:"responseTime"`
	StatusCode       int      `json:"statusCode,omitempty"`
	TargetURL        string   `json:"targetUrl,omitempty"`
	ContentType      string   `json:"contentType,omitempty"`
	Remaining        *float64 `json:"remaining,omitempty"`
	Total            *float64 `json:"total,omitempty"`
	Used             *float64 `json:"used,omitempty"`
	Unit             string   `json:"unit,omitempty"`
	Plan             string   `json:"plan,omitempty"`
	Valid            *bool    `json:"valid,omitempty"`
	Message          string   `json:"message,omitempty"`
	CheckedTime      int64    `json:"checkedTime,omitempty"`
	SuccessTime      int64    `json:"successTime,omitempty"`
	htmlResponse     bool
}

type ProviderBalanceRefreshSummary struct {
	Due       int
	Succeeded int
	Failed    int
	Skipped   bool
}

var providerBalanceRefreshRunning atomic.Bool

func (s *ProviderService) Balance(guid string) (*ProviderBalanceResult, error) {
	return s.BalanceContext(context.Background(), guid)
}

func (s *ProviderService) BalanceContext(ctx context.Context, guid string) (*ProviderBalanceResult, error) {
	provider, err := s.GetByGUID(guid)
	if err != nil {
		return nil, err
	}
	return s.refreshProviderBalance(ctx, provider)
}

func (s *ProviderService) TestBalance(provider *domains.VendorMeta) (*ProviderBalanceResult, error) {
	if provider == nil {
		return nil, errors.New("provider is required")
	}
	s.fillProviderSecretFields(provider)
	result, err := s.queryBalance(context.Background(), provider)
	if err != nil || !shouldProbeSub2Balance(provider, result) {
		return result, err
	}

	sub2Provider := *provider
	applySub2BalanceDefaults(&sub2Provider)
	sub2TargetURL, targetErr := sub2BalanceProbeURL(result.TargetURL)
	if targetErr != nil {
		return result, nil
	}
	sub2Provider.BalanceCustomPath = sub2TargetURL
	detected, detectErr := s.queryBalance(context.Background(), &sub2Provider)
	if detectErr == nil && detected.OK {
		detected.DetectedTemplate = balanceTemplateSub2
		return detected, nil
	}
	return result, nil
}

func (s *ProviderService) queryBalance(ctx context.Context, provider *domains.VendorMeta) (*ProviderBalanceResult, error) {
	if provider == nil {
		return nil, errors.New("provider is required")
	}
	normalizeProviderBalanceConfig(provider)
	normalizeProviderProxyConfig(provider)
	if err := validateProviderProxyConfig(provider); err != nil {
		return nil, err
	}
	targetURL, err := providerBalanceTargetURL(provider)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	setupBalanceHeaders(req.Header, provider)
	client, err := providerHTTPClient(provider, 12*time.Second)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	resp, err := client.Do(req)
	responseTime := time.Since(start).Milliseconds()
	if err != nil {
		return &ProviderBalanceResult{
			OK:           false,
			Template:     provider.BalanceTemplate,
			ResponseTime: responseTime,
			TargetURL:    targetURL,
			Message:      err.Error(),
		}, nil
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if readErr != nil {
		return nil, readErr
	}
	result := &ProviderBalanceResult{
		Template:     provider.BalanceTemplate,
		ResponseTime: responseTime,
		StatusCode:   resp.StatusCode,
		TargetURL:    targetURL,
		ContentType:  strings.TrimSpace(resp.Header.Get("Content-Type")),
		Unit:         strings.TrimSpace(provider.BalanceUnit),
	}
	if result.ContentType == "" {
		result.ContentType = http.DetectContentType(body)
	}
	result.htmlResponse = isHTMLBalanceResponse(result.ContentType, body)
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		result.OK = false
		if result.htmlResponse {
			result.Message = "余额接口返回了网页内容；该服务可能是 Sub2API，请使用 Sub2API 模板（/v1/usage）"
		} else {
			result.Message = clipBalanceMessage(string(body))
		}
		if result.Message == "" {
			result.Message = err.Error()
		}
		return result, nil
	}
	applyBalancePayload(result, provider, payload)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		result.OK = false
		if result.Message == "" {
			result.Message = fmt.Sprintf("upstream returned %d", resp.StatusCode)
		}
		return result, nil
	}
	if result.Valid != nil && !*result.Valid {
		result.OK = false
		if result.Message == "" {
			result.Message = "balance response marked invalid"
		}
		return result, nil
	}
	if result.Remaining == nil && result.Total == nil && result.Used == nil {
		result.OK = false
		if result.Message == "" {
			result.Message = "未解析到余额字段，请检查 JSON 路径"
		}
		return result, nil
	}
	result.OK = true
	if result.Message == "" {
		result.Message = "balance query succeeded"
	}
	return result, nil
}

func (s *ProviderService) RefreshDueBalances(ctx context.Context) (ProviderBalanceRefreshSummary, error) {
	if !providerBalanceRefreshRunning.CompareAndSwap(false, true) {
		return ProviderBalanceRefreshSummary{Skipped: true}, nil
	}
	defer providerBalanceRefreshRunning.Store(false)
	if ctx == nil {
		ctx = context.Background()
	}
	intervalSeconds := OptionServiceApp.Int64("provider.balance_refresh_interval_seconds", int64(defaultBalanceRefreshInterval/time.Second))
	if intervalSeconds <= 0 {
		intervalSeconds = int64(defaultBalanceRefreshInterval / time.Second)
	}
	workers := OptionServiceApp.Int64("provider.balance_refresh_workers", defaultBalanceRefreshWorkers)
	if workers <= 0 {
		workers = defaultBalanceRefreshWorkers
	}
	if workers > 16 {
		workers = 16
	}
	cutoff := time.Now().Add(-time.Duration(intervalSeconds) * time.Second).UnixMilli()
	db := s.DB()
	if db == nil {
		return ProviderBalanceRefreshSummary{}, errors.New("database is not initialized")
	}
	var providers []domains.VendorMeta
	if err := db.
		Where("balance_check_enabled = ? AND (balance_checked_time = 0 OR balance_checked_time <= ?)", true, cutoff).
		Order("balance_checked_time asc, id asc").
		Find(&providers).Error; err != nil {
		return ProviderBalanceRefreshSummary{}, err
	}
	summary := ProviderBalanceRefreshSummary{Due: len(providers)}
	if len(providers) == 0 {
		return summary, nil
	}
	jobs := make(chan *domains.VendorMeta)
	var waitGroup sync.WaitGroup
	var mu sync.Mutex
	for range workers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for provider := range jobs {
				result, err := s.refreshProviderBalance(ctx, provider)
				mu.Lock()
				if err == nil && result != nil && result.OK {
					summary.Succeeded++
				} else {
					summary.Failed++
				}
				mu.Unlock()
			}
		}()
	}
	for i := range providers {
		select {
		case jobs <- &providers[i]:
		case <-ctx.Done():
			close(jobs)
			waitGroup.Wait()
			return summary, ctx.Err()
		}
	}
	close(jobs)
	waitGroup.Wait()
	return summary, nil
}

func (s *ProviderService) refreshProviderBalance(ctx context.Context, provider *domains.VendorMeta) (*ProviderBalanceResult, error) {
	if provider == nil {
		return nil, errors.New("provider is required")
	}
	if !provider.BalanceCheckEnabled {
		return nil, errors.New("provider balance check is disabled")
	}
	db := s.DB()
	if db == nil {
		return nil, errors.New("database is not initialized")
	}
	result, queryErr := s.queryBalance(ctx, provider)
	checkedTime := time.Now().UnixMilli()
	if result == nil {
		result = &ProviderBalanceResult{
			OK:          false,
			Template:    provider.BalanceTemplate,
			Message:     errorMessage(queryErr),
			CheckedTime: checkedTime,
		}
	}
	result.CheckedTime = checkedTime
	updates := providerBalanceSnapshotUpdates(provider, result)
	if queryErr != nil && result.Message == "" {
		result.Message = queryErr.Error()
		updates["balance_query_message"] = clipBalanceMessage(queryErr.Error())
	}
	if err := db.Model(&domains.VendorMeta{}).Where("id = ?", provider.Id).Updates(updates).Error; err != nil {
		return result, err
	}
	if result.OK {
		result.SuccessTime = checkedTime
	} else {
		result.SuccessTime = provider.BalanceSuccessTime
	}
	return result, queryErr
}

func providerBalanceSnapshotUpdates(provider *domains.VendorMeta, result *ProviderBalanceResult) map[string]any {
	checkedTime := result.CheckedTime
	if checkedTime <= 0 {
		checkedTime = time.Now().UnixMilli()
	}
	updates := map[string]any{
		"balance_query_ok":         result.OK,
		"balance_query_message":    clipBalanceMessage(result.Message),
		"balance_status_code":      result.StatusCode,
		"balance_response_time_ms": result.ResponseTime,
		"balance_checked_time":     checkedTime,
	}
	if result.OK {
		updates["balance_remaining"] = result.Remaining
		updates["balance_total"] = result.Total
		updates["balance_used"] = result.Used
		updates["balance_snapshot_unit"] = strings.TrimSpace(result.Unit)
		updates["balance_snapshot_plan"] = clipBalancePlan(result.Plan)
		updates["balance_success_time"] = checkedTime
	} else if provider.BalanceSuccessTime <= 0 {
		updates["balance_remaining"] = nil
		updates["balance_total"] = nil
		updates["balance_used"] = nil
		updates["balance_snapshot_unit"] = ""
		updates["balance_snapshot_plan"] = ""
	}
	return updates
}

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func clipBalancePlan(plan string) string {
	return clipBalanceText(plan, 255)
}

func shouldProbeSub2Balance(provider *domains.VendorMeta, result *ProviderBalanceResult) bool {
	if provider == nil || result == nil || result.OK || result.StatusCode != http.StatusOK {
		return false
	}
	if normalizeBalanceTemplate(provider.BalanceTemplate) != balanceTemplateGeneric {
		return false
	}
	if strings.TrimSpace(provider.BalanceCustomPath) != defaultBalancePath(balanceTemplateGeneric) {
		return false
	}
	return result.htmlResponse || strings.Contains(strings.ToLower(result.ContentType), "text/html")
}

func applySub2BalanceDefaults(provider *domains.VendorMeta) {
	provider.BalanceTemplate = balanceTemplateSub2
	provider.BalanceCustomPath = defaultBalancePath(balanceTemplateSub2)
	provider.BalanceAuthType = balanceAuthProviderBearer
	provider.BalanceRemainingPath = "remaining"
	provider.BalanceMultiplier = 1
	provider.BalanceUnit = "USD"
	provider.BalanceTotalPath = "quota.limit"
	provider.BalanceUsedPath = "quota.used"
	provider.BalancePlanPath = "planName"
	provider.BalanceValidPath = "isValid"
	provider.BalanceErrorPath = "message"
}

func sub2BalanceProbeURL(sourceURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(sourceURL))
	if err != nil || parsed.Host == "" {
		return "", errors.New("balance response url is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("balance response url only supports http and https")
	}
	parsed.User = nil
	parsed.Path = defaultBalancePath(balanceTemplateSub2)
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	return parsed.String(), nil
}

func isHTMLBalanceResponse(contentType string, body []byte) bool {
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		return true
	}
	text := strings.ToLower(strings.TrimSpace(string(body)))
	return strings.HasPrefix(text, "<!doctype html") || strings.HasPrefix(text, "<html")
}

func (s *ProviderService) fillProviderSecretFields(provider *domains.VendorMeta) {
	if provider == nil || strings.TrimSpace(provider.Guid) == "" {
		return
	}
	existing, err := s.GetByGUID(provider.Guid)
	if err != nil || existing == nil {
		return
	}
	if strings.TrimSpace(provider.Key) == "" {
		provider.Key = existing.Key
	}
	if strings.TrimSpace(provider.ProxyPassword) == "" {
		provider.ProxyPassword = existing.ProxyPassword
	}
	if strings.TrimSpace(provider.BalanceAccessToken) == "" {
		provider.BalanceAccessToken = existing.BalanceAccessToken
	}
}

func setupBalanceHeaders(header http.Header, provider *domains.VendorMeta) {
	header.Set("Accept", "application/json")
	header.Set("User-Agent", balanceUserAgent(provider.BalanceTemplate))
	switch strings.ToLower(strings.TrimSpace(provider.BalanceAuthType)) {
	case balanceAuthNone:
		return
	case balanceAuthAccessToken:
		if token := strings.TrimSpace(provider.BalanceAccessToken); token != "" {
			header.Set("Authorization", "Bearer "+token)
		}
	default:
		if token := strings.TrimSpace(provider.BalanceAccessToken); token != "" {
			header.Set("Authorization", "Bearer "+token)
			return
		}
		if token := strings.TrimSpace(ProviderServiceApp.NextKey(provider)); token != "" {
			header.Set("Authorization", "Bearer "+token)
		}
	}
}

func balanceUserAgent(template string) string {
	if normalizeBalanceTemplate(template) == balanceTemplateGeneric {
		return "cc-switch/1.0"
	}
	return "NavAPI Balance Probe"
}

func providerBalanceTargetURL(provider *domains.VendorMeta) (string, error) {
	path := strings.TrimSpace(provider.BalanceCustomPath)
	if path == "" || path == "/v1/usage" {
		path = defaultBalancePath(provider.BalanceTemplate)
	}
	if strings.Contains(path, "://") {
		return normalizeHTTPURL(path)
	}
	baseURL := strings.TrimSpace(provider.BalanceBaseURL)
	explicitBalanceBase := baseURL != ""
	if baseURL == "" {
		baseURL = strings.TrimSpace(provider.BaseURL)
	}
	if baseURL == "" && provider.BalanceTemplate == balanceTemplateOfficial {
		baseURL = "https://api.openai.com"
	}
	if baseURL == "" {
		return "", errors.New("balance base url is required")
	}
	normalized, err := normalizeHTTPURL(baseURL)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return "", err
	}
	if !explicitBalanceBase || provider.BalanceTemplate == balanceTemplateOfficial || provider.BalanceTemplate == balanceTemplateSub2 {
		parsed.Path = stripAPIVersionSuffix(parsed.Path)
		parsed.RawQuery = ""
	}
	parsed.Path = joinURLPath(parsed.Path, path)
	return parsed.String(), nil
}

func normalizeHTTPURL(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", errors.New("url is required")
	}
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || strings.TrimSpace(parsed.Host) == "" {
		return "", errors.New("url is invalid")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		return "", errors.New("url only supports http and https")
	}
	return parsed.String(), nil
}

func stripAPIVersionSuffix(path string) string {
	path = strings.TrimRight(strings.TrimSpace(path), "/")
	lower := strings.ToLower(path)
	for _, suffix := range []string{"/v1", "/v1beta", "/v1beta1", "/v2"} {
		if lower == suffix {
			return ""
		}
		if strings.HasSuffix(lower, suffix) {
			return path[:len(path)-len(suffix)]
		}
	}
	return path
}

func joinURLPath(basePath string, nextPath string) string {
	nextPath = strings.TrimSpace(nextPath)
	if nextPath == "" {
		if basePath == "" {
			return "/"
		}
		return basePath
	}
	if !strings.HasPrefix(nextPath, "/") {
		nextPath = "/" + nextPath
	}
	basePath = strings.TrimRight(strings.TrimSpace(basePath), "/")
	if basePath == "" {
		return nextPath
	}
	return basePath + nextPath
}

func applyBalancePayload(result *ProviderBalanceResult, provider *domains.VendorMeta, payload any) {
	multiplier := provider.BalanceMultiplier
	if multiplier <= 0 || math.IsNaN(multiplier) || math.IsInf(multiplier, 0) {
		multiplier = 1
	}
	result.Remaining = balanceNumber(payload, multiplier, configuredPath(provider.BalanceRemainingPath), balanceRemainingPaths(provider.BalanceTemplate)...)
	result.Total = balanceNumber(payload, multiplier, configuredPath(provider.BalanceTotalPath), balanceTotalPaths(provider.BalanceTemplate)...)
	result.Used = balanceNumber(payload, multiplier, configuredPath(provider.BalanceUsedPath), balanceUsedPaths(provider.BalanceTemplate)...)
	if result.Total == nil && result.Remaining != nil && result.Used != nil {
		total := *result.Remaining + *result.Used
		result.Total = &total
	}
	if value, ok := firstJSONValue(payload, appendPath(configuredPath(provider.BalancePlanPath), balancePlanPaths(provider.BalanceTemplate)...)...); ok {
		result.Plan = stringifyJSONValue(value)
	}
	if value, ok := firstJSONValue(payload, appendPath(configuredPath(provider.BalanceValidPath), balanceValidPaths(provider.BalanceTemplate)...)...); ok {
		if valid, ok := boolFromJSONValue(value); ok {
			result.Valid = &valid
		}
	}
	if value, ok := firstJSONValue(payload, appendPath(configuredPath(provider.BalanceErrorPath), balanceErrorPaths()...)...); ok {
		result.Message = stringifyJSONValue(value)
	}
}

func configuredPath(path string) []string {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	return []string{path}
}

func appendPath(first []string, rest ...string) []string {
	out := make([]string, 0, len(first)+len(rest))
	out = append(out, first...)
	out = append(out, rest...)
	return out
}

func balanceNumber(payload any, multiplier float64, first []string, rest ...string) *float64 {
	if value, ok := firstJSONValue(payload, appendPath(first, rest...)...); ok {
		if number, ok := numberFromJSONValue(value); ok {
			number *= multiplier
			return &number
		}
	}
	return nil
}

func firstJSONValue(payload any, paths ...string) (any, bool) {
	seen := map[string]struct{}{}
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		if value, ok := jsonPathValue(payload, path); ok {
			return value, true
		}
	}
	return nil, false
}

func jsonPathValue(payload any, path string) (any, bool) {
	current := payload
	for _, part := range jsonPathParts(path) {
		if part == "" {
			continue
		}
		switch node := current.(type) {
		case map[string]any:
			value, ok := node[part]
			if !ok {
				return nil, false
			}
			current = value
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(node) {
				return nil, false
			}
			current = node[index]
		default:
			return nil, false
		}
	}
	return current, true
}

func jsonPathParts(path string) []string {
	path = strings.TrimSpace(path)
	path = strings.ReplaceAll(path, "[", ".")
	path = strings.ReplaceAll(path, "]", "")
	return strings.Split(path, ".")
}

func numberFromJSONValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	case string:
		number, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return number, err == nil
	default:
		return 0, false
	}
}

func boolFromJSONValue(value any) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "1", "yes", "ok", "active", "valid":
			return true, true
		case "false", "0", "no", "inactive", "invalid":
			return false, true
		default:
			return false, false
		}
	default:
		number, ok := numberFromJSONValue(value)
		if !ok {
			return false, false
		}
		return number != 0, true
	}
}

func stringifyJSONValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case map[string]any, []any:
		body, err := json.Marshal(typed)
		if err != nil {
			return ""
		}
		return string(body)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func clipBalanceMessage(message string) string {
	return clipBalanceText(message, 500)
}

func clipBalanceText(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if limit > 0 && len(runes) > limit {
		return string(runes[:limit])
	}
	return value
}

func defaultBalancePath(template string) string {
	switch normalizeBalanceTemplate(template) {
	case balanceTemplateNewAPI:
		return "/api/user/self"
	case balanceTemplateSub2:
		return "/v1/usage"
	case balanceTemplateOfficial:
		return "/dashboard/billing/credit_grants"
	default:
		return "/user/balance"
	}
}

func defaultBalanceRemainingPath(template string) string {
	switch normalizeBalanceTemplate(template) {
	case balanceTemplateNewAPI:
		return "data.quota"
	case balanceTemplateOfficial:
		return "total_available"
	default:
		return "remaining"
	}
}

func defaultBalanceUnit(template string) string {
	switch normalizeBalanceTemplate(template) {
	case balanceTemplateNewAPI, balanceTemplateOfficial:
		return "USD"
	default:
		return "USD"
	}
}

func defaultBalanceMultiplier(template string) float64 {
	if normalizeBalanceTemplate(template) == balanceTemplateNewAPI {
		return defaultNewAPIQuotaMultiplier
	}
	return 1
}

func balanceRemainingPaths(template string) []string {
	switch normalizeBalanceTemplate(template) {
	case balanceTemplateNewAPI:
		return []string{"data.quota", "quota", "data.remaining_quota", "remaining_quota", "data.balance", "balance"}
	case balanceTemplateSub2:
		return []string{"remaining", "quota.remaining", "balance"}
	case balanceTemplateOfficial:
		return []string{"total_available", "data.total_available", "credit_grants.total_available", "remaining", "balance"}
	default:
		return []string{"remaining", "balance", "data.remaining", "data.balance", "data.quota", "quota", "remain", "available"}
	}
}

func balanceTotalPaths(template string) []string {
	switch normalizeBalanceTemplate(template) {
	case balanceTemplateNewAPI:
		return []string{"data.total_quota", "total_quota", "data.total", "total"}
	case balanceTemplateSub2:
		return []string{"quota.limit", "total", "balance"}
	case balanceTemplateOfficial:
		return []string{"total_granted", "data.total_granted", "credit_grants.total_granted", "total"}
	default:
		return []string{"total", "data.total", "total_quota", "data.total_quota"}
	}
}

func balanceUsedPaths(template string) []string {
	switch normalizeBalanceTemplate(template) {
	case balanceTemplateNewAPI:
		return []string{"data.used_quota", "used_quota", "data.used", "used"}
	case balanceTemplateSub2:
		return []string{"quota.used", "used", "usage.total.actual_cost"}
	case balanceTemplateOfficial:
		return []string{"total_used", "data.total_used", "credit_grants.total_used", "used"}
	default:
		return []string{"used", "data.used", "used_quota", "data.used_quota", "consumed", "data.consumed"}
	}
}

func balancePlanPaths(template string) []string {
	switch normalizeBalanceTemplate(template) {
	case balanceTemplateNewAPI:
		return []string{"data.group", "group", "data.role", "role", "data.plan", "plan"}
	case balanceTemplateSub2:
		return []string{"planName", "mode"}
	case balanceTemplateOfficial:
		return []string{"plan", "data.plan"}
	default:
		return []string{"plan", "data.plan", "package", "data.package"}
	}
}

func balanceValidPaths(template string) []string {
	switch normalizeBalanceTemplate(template) {
	case balanceTemplateNewAPI:
		return []string{"success", "data.status", "status", "data.enabled", "enabled"}
	case balanceTemplateSub2:
		return []string{"isValid", "valid", "active"}
	default:
		return []string{"valid", "active", "data.valid", "data.active", "success"}
	}
}

func balanceErrorPaths() []string {
	return []string{"error.message", "error", "message", "msg", "data.message", "data.error"}
}
