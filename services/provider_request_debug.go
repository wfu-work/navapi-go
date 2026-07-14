package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"navapi-go/domains"

	"golang.org/x/net/http/httpguts"
)

const (
	providerRequestDebugDefaultTimeout = 30
	providerRequestDebugMaxTimeout     = 120
	providerRequestDebugMaxBodyBytes   = 256 * 1024
	providerRequestDebugMaxResultBytes = 1024 * 1024
)

var providerRequestDebugMethods = map[string]struct{}{
	http.MethodGet:    {},
	http.MethodPost:   {},
	http.MethodPut:    {},
	http.MethodPatch:  {},
	http.MethodDelete: {},
	http.MethodHead:   {},
}

type ProviderRequestDebugInput struct {
	ProviderGUID   string            `json:"providerGuid"`
	Method         string            `json:"method"`
	URL            string            `json:"url"`
	AuthType       string            `json:"authType"`
	AuthName       string            `json:"authName"`
	Token          string            `json:"token"`
	Query          map[string]string `json:"query"`
	Headers        map[string]string `json:"headers"`
	Body           string            `json:"body"`
	TimeoutSeconds int               `json:"timeoutSeconds"`
	FollowRedirect bool              `json:"followRedirect"`
}

type ProviderRequestDebugResult struct {
	OK             bool                `json:"ok"`
	Method         string              `json:"method"`
	TargetURL      string              `json:"targetUrl"`
	StatusCode     int                 `json:"statusCode,omitempty"`
	ResponseTimeMs int64               `json:"responseTimeMs"`
	ContentType    string              `json:"contentType,omitempty"`
	ResponseBytes  int                 `json:"responseBytes"`
	Truncated      bool                `json:"truncated"`
	Headers        map[string][]string `json:"headers,omitempty"`
	Response       any                 `json:"response,omitempty"`
	Message        string              `json:"message"`
}

func (s *ProviderService) DebugRequest(ctx context.Context, input ProviderRequestDebugInput) (*ProviderRequestDebugResult, error) {
	providerGUID := strings.TrimSpace(input.ProviderGUID)
	provider := new(domains.VendorMeta)
	if providerGUID != "" {
		storedProvider, err := s.GetByGUID(providerGUID)
		if err != nil {
			return nil, err
		}
		provider = storedProvider
	}
	return executeProviderRequestDebug(ctx, provider, input)
}

func executeProviderRequestDebug(ctx context.Context, provider *domains.VendorMeta, input ProviderRequestDebugInput) (*ProviderRequestDebugResult, error) {
	if provider == nil {
		return nil, errors.New("provider is required")
	}
	method := strings.ToUpper(strings.TrimSpace(input.Method))
	if _, ok := providerRequestDebugMethods[method]; !ok {
		return nil, errors.New("method only supports GET, POST, PUT, PATCH, DELETE and HEAD")
	}
	target, err := providerRequestDebugURL(input.URL)
	if err != nil {
		return nil, err
	}
	if err := applyProviderDebugQuery(target, input.Query); err != nil {
		return nil, err
	}
	if len(input.Body) > providerRequestDebugMaxBodyBytes {
		return nil, fmt.Errorf("request body exceeds %d KB", providerRequestDebugMaxBodyBytes/1024)
	}

	authType := strings.ToLower(strings.TrimSpace(input.AuthType))
	authName := strings.TrimSpace(input.AuthName)
	token := strings.TrimSpace(input.Token)
	if token == "" {
		token = strings.TrimSpace(provider.Key)
	}
	if err := applyProviderDebugQueryAuth(target, authType, authName, token); err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, method, target.String(), strings.NewReader(input.Body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	if input.Body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if err := applyProviderDebugHeaderAuth(request.Header, authType, authName, token); err != nil {
		return nil, err
	}
	if err := applyProviderDebugHeaders(request.Header, input.Headers); err != nil {
		return nil, err
	}

	timeoutSeconds := input.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = providerRequestDebugDefaultTimeout
	}
	if timeoutSeconds > providerRequestDebugMaxTimeout {
		timeoutSeconds = providerRequestDebugMaxTimeout
	}
	normalizeProviderProxyConfig(provider)
	if err := validateProviderProxyConfig(provider); err != nil {
		return nil, err
	}
	client, err := providerHTTPClient(provider, time.Duration(timeoutSeconds)*time.Second)
	if err != nil {
		return nil, err
	}
	if !input.FollowRedirect {
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	result := &ProviderRequestDebugResult{
		Method:    method,
		TargetURL: providerRequestDebugDisplayURL(target, authType, authName),
	}
	start := time.Now()
	response, requestErr := client.Do(request)
	result.ResponseTimeMs = time.Since(start).Milliseconds()
	if requestErr != nil {
		result.Message = requestErr.Error()
		return result, nil
	}
	defer response.Body.Close()

	result.StatusCode = response.StatusCode
	result.ContentType = strings.TrimSpace(response.Header.Get("Content-Type"))
	result.Headers = response.Header.Clone()
	body, err := io.ReadAll(io.LimitReader(response.Body, providerRequestDebugMaxResultBytes+1))
	if err != nil {
		return nil, err
	}
	result.ResponseBytes = len(body)
	if len(body) > providerRequestDebugMaxResultBytes {
		result.Truncated = true
		body = body[:providerRequestDebugMaxResultBytes]
	}
	result.Response = providerRequestDebugResponse(body, result.ContentType)
	result.OK = response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices
	if result.OK {
		result.Message = "上游请求成功"
	} else {
		result.Message = providerDebugErrorMessage(result.Response)
		if result.Message == "" {
			result.Message = response.Status
		}
	}
	return result, nil
}

func providerRequestDebugURL(raw string) (*url.URL, error) {
	target, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || target.Host == "" {
		return nil, errors.New("request url is invalid")
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return nil, errors.New("request url only supports http and https")
	}
	if target.User != nil {
		return nil, errors.New("request url must not contain user info")
	}
	return target, nil
}

func applyProviderDebugQuery(target *url.URL, values map[string]string) error {
	query := target.Query()
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			return errors.New("query parameter name is required")
		}
		query.Set(key, value)
	}
	target.RawQuery = query.Encode()
	return nil
}

func applyProviderDebugQueryAuth(target *url.URL, authType string, authName string, token string) error {
	if authType != "query" {
		return validateProviderDebugAuth(authType, authName, token)
	}
	if err := validateProviderDebugAuth(authType, authName, token); err != nil {
		return err
	}
	query := target.Query()
	query.Set(authName, token)
	target.RawQuery = query.Encode()
	return nil
}

func applyProviderDebugHeaderAuth(headers http.Header, authType string, authName string, token string) error {
	if err := validateProviderDebugAuth(authType, authName, token); err != nil {
		return err
	}
	switch authType {
	case "", "none", "query":
		return nil
	case "bearer":
		headers.Set("Authorization", "Bearer "+token)
	case "header":
		headers.Set(authName, token)
	}
	return nil
}

func validateProviderDebugAuth(authType string, authName string, token string) error {
	switch authType {
	case "", "none":
		return nil
	case "bearer":
		if token == "" {
			return errors.New("token is required")
		}
		return nil
	case "header", "query":
		if token == "" {
			return errors.New("token is required")
		}
		if !validProviderDebugName(authName) {
			return errors.New("auth parameter name is invalid")
		}
		return nil
	default:
		return errors.New("auth type only supports none, bearer, header and query")
	}
}

func applyProviderDebugHeaders(headers http.Header, values map[string]string) error {
	for key, value := range values {
		key = strings.TrimSpace(key)
		if !validProviderDebugName(key) || !httpguts.ValidHeaderFieldValue(value) {
			return fmt.Errorf("request header %q is invalid", key)
		}
		switch strings.ToLower(key) {
		case "host", "content-length", "transfer-encoding", "connection", "upgrade":
			return fmt.Errorf("request header %q cannot be overridden", key)
		}
		headers.Set(key, value)
	}
	return nil
}

func validProviderDebugName(value string) bool {
	return value != "" && httpguts.ValidHeaderFieldName(value)
}

func providerRequestDebugDisplayURL(target *url.URL, authType string, authName string) string {
	copyURL := *target
	if authType == "query" && authName != "" {
		query := copyURL.Query()
		if query.Has(authName) {
			query.Set(authName, "******")
			copyURL.RawQuery = query.Encode()
		}
	}
	return copyURL.String()
}

func providerRequestDebugResponse(body []byte, contentType string) any {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	var value any
	if json.Unmarshal(body, &value) == nil {
		return value
	}
	return map[string]any{
		"contentType": contentType,
		"text":        string(body),
	}
}
