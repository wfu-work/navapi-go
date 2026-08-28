package apis

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"navapi-go/constants"
	"navapi-go/domains"
	"navapi-go/services"
	"navapi-go/vos"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type RelayApi struct{}

type tokenBalanceResponse struct {
	IsActive  bool     `json:"is_active"`
	Name      string   `json:"name"`
	Balance   *float64 `json:"balance"`
	Used      float64  `json:"used"`
	Total     *float64 `json:"total"`
	Unlimited bool     `json:"unlimited"`
	Unit      string   `json:"unit"`
}

func (a RelayApi) TokenBalance(c *gin.Context) {
	token, ok := c.MustGet(constants.ContextToken).(*domains.ApiToken)
	if !ok || token == nil {
		openAIError(c, http.StatusUnauthorized, "token is invalid")
		return
	}
	wallet, err := userWalletService.Get(token.UserGuid)
	if err != nil {
		openAIError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, buildTokenBalanceResponse(token, wallet))
}

func buildTokenBalanceResponse(token *domains.ApiToken, wallet *domains.UserWallet) tokenBalanceResponse {
	used := services.AmountMicrosToCost(token.UsedAmountMicros)
	availableMicros := token.BalanceAmountMicros
	effectiveUnlimited := token.UnlimitedBalance
	unit := "CNY"
	if wallet != nil {
		// UnlimitedBalance only removes the per-token cap. The user's wallet is
		// still finite, so balance clients such as CCS must see the wallet amount
		// instead of treating the whole account as unlimited.
		effectiveUnlimited = false
		walletBalance := wallet.BalanceAmountMicros
		if token.UnlimitedBalance || walletBalance < availableMicros {
			availableMicros = walletBalance
		}
		if currency := strings.TrimSpace(wallet.Currency); currency != "" {
			unit = currency
		}
	}
	available := services.AmountMicrosToCost(availableMicros)
	// CCS generic extractors use balance || total, so total is a remaining-balance alias.
	total := available
	result := tokenBalanceResponse{
		IsActive:  token.Status == constants.StatusEnabled,
		Name:      token.Name,
		Balance:   &available,
		Used:      used,
		Total:     &total,
		Unlimited: effectiveUnlimited,
		Unit:      unit,
	}
	return result
}

func (a RelayApi) Models(c *gin.Context) {
	group := constants.DefaultGroup
	var apiToken *domains.ApiToken
	if token, ok := c.Get(constants.ContextToken); ok {
		apiToken, _ = token.(*domains.ApiToken)
		if apiToken != nil {
			group = apiToken.Group
		}
	}
	models, err := modelService.ListOpenAIModelsForGroup(group)
	if err != nil {
		openAIError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if apiToken != nil {
		filtered := models.Data[:0]
		for _, model := range models.Data {
			if tokenService.CheckModel(apiToken, model.ID) == nil {
				filtered = append(filtered, model)
			}
		}
		models.Data = filtered
	}
	if modelID := c.Param("model"); modelID != "" {
		for _, model := range models.Data {
			if model.ID == modelID {
				c.JSON(http.StatusOK, model)
				return
			}
		}
		openAIError(c, http.StatusNotFound, "model not found")
		return
	}
	c.JSON(http.StatusOK, models)
}

func (a RelayApi) ChatCompletions(c *gin.Context) {
	a.Relay(c, services.RelayEndpoint{
		UpstreamPath: "/v1/chat/completions",
		Method:       http.MethodPost,
		Format:       constants.ProviderTypeOpenAI,
	})
}

func (a RelayApi) Relay(c *gin.Context, endpoint services.RelayEndpoint) {
	token, ok := c.MustGet(constants.ContextToken).(*domains.ApiToken)
	if !ok || token == nil {
		openAIError(c, http.StatusUnauthorized, "token is invalid")
		return
	}
	ensureRelayRequestID(c)
	result, streamed, err := relayService.RelayHTTP(c, token, endpoint)
	if err != nil {
		if streamed && c.Writer.Written() {
			return
		}
		var relayErr *services.RelayHTTPError
		if errors.As(err, &relayErr) {
			openAIRelayError(c, relayErr)
			return
		}
		openAIError(c, http.StatusBadGateway, err.Error())
		return
	}
	if streamed {
		return
	}
	responseHeaders := services.NormalizeRelayResponseHeaders(result.StatusCode, result.Header)
	for key, values := range responseHeaders {
		if key == "Content-Length" || key == "Transfer-Encoding" {
			continue
		}
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}
	contentType := responseHeaders.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(result.StatusCode, contentType, result.Body)
}

func (a RelayApi) OpenAICompletions(c *gin.Context) {
	a.Relay(c, services.RelayEndpoint{UpstreamPath: "/v1/completions", Method: http.MethodPost, Format: constants.ProviderTypeOpenAI})
}

func (a RelayApi) OpenAIEmbeddings(c *gin.Context) {
	a.Relay(c, services.RelayEndpoint{UpstreamPath: "/v1/embeddings", Method: http.MethodPost, Format: constants.ProviderTypeOpenAI})
}

func (a RelayApi) OpenAIResponses(c *gin.Context) {
	a.Relay(c, services.RelayEndpoint{UpstreamPath: "/v1/responses", Method: http.MethodPost, Format: constants.ProviderTypeOpenAI})
}

func (a RelayApi) OpenAIModerations(c *gin.Context) {
	a.Relay(c, services.RelayEndpoint{UpstreamPath: "/v1/moderations", Method: http.MethodPost, Format: constants.ProviderTypeOpenAI, DefaultModel: "omni-moderation-latest"})
}

func (a RelayApi) OpenAIRerank(c *gin.Context) {
	a.Relay(c, services.RelayEndpoint{UpstreamPath: "/v1/rerank", Method: http.MethodPost, Format: constants.ProviderTypeOpenAI, DefaultModel: "rerank"})
}

func (a RelayApi) OpenAIImageGenerations(c *gin.Context) {
	a.Relay(c, services.RelayEndpoint{UpstreamPath: "/v1/images/generations", Method: http.MethodPost, Format: constants.ProviderTypeOpenAI})
}

func (a RelayApi) OpenAIAudioTranscriptions(c *gin.Context) {
	a.Relay(c, services.RelayEndpoint{UpstreamPath: "/v1/audio/transcriptions", Method: http.MethodPost, Format: constants.ProviderTypeOpenAI})
}

func (a RelayApi) OpenAIAudioTranslations(c *gin.Context) {
	a.Relay(c, services.RelayEndpoint{UpstreamPath: "/v1/audio/translations", Method: http.MethodPost, Format: constants.ProviderTypeOpenAI})
}

func (a RelayApi) ClaudeMessages(c *gin.Context) {
	a.Relay(c, services.RelayEndpoint{UpstreamPath: "/v1/messages", Method: http.MethodPost, Format: constants.ProviderTypeAnthropic})
}

func (a RelayApi) GeminiModels(c *gin.Context) {
	upstreamPath := "/v1beta/models/" + strings.TrimPrefix(c.Param("path"), "/")
	a.Relay(c, services.RelayEndpoint{
		UpstreamPath:  upstreamPath,
		Method:        http.MethodPost,
		Format:        constants.ProviderTypeGemini,
		ModelFromPath: true,
	})
}

func (a RelayApi) MidjourneyTask(c *gin.Context) {
	a.AsyncTask(c, "midjourney")
}

func (a RelayApi) SunoTask(c *gin.Context) {
	a.AsyncTask(c, "suno")
}

func (a RelayApi) AsyncTask(c *gin.Context, platform string) {
	token, ok := c.MustGet(constants.ContextToken).(*domains.ApiToken)
	if !ok || token == nil {
		openAIError(c, http.StatusUnauthorized, "token is invalid")
		return
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		openAIError(c, http.StatusBadRequest, err.Error())
		return
	}
	task := domains.Task{
		Platform: platform,
		UserGuid: token.UserGuid,
		Group:    token.Group,
		Action:   c.Param("path"),
		Status:   "submitted",
		Data:     string(body),
	}
	if err := taskService.Create(&task); err != nil {
		openAIError(c, http.StatusBadGateway, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"task_id":  task.TaskID,
		"status":   task.Status,
		"platform": task.Platform,
	})
}

func openAIError(c *gin.Context, code int, message string) {
	c.JSON(code, vos.OpenAIErrorResponse{Error: vos.OpenAIError{
		Message: message,
		Type:    "invalid_request_error",
	}})
}

func openAIRelayError(c *gin.Context, relayErr *services.RelayHTTPError) {
	if relayErr == nil {
		openAIError(c, http.StatusBadGateway, "relay request failed")
		return
	}
	statusCode := relayErr.StatusCode
	if statusCode <= 0 {
		statusCode = http.StatusBadGateway
	}
	retryAfterSeconds := retryAfterHeaderSeconds(relayErr.RetryAfter)
	if retryAfterSeconds > 0 {
		c.Header("Retry-After", strconv.FormatInt(retryAfterSeconds, 10))
	}
	requestID := strings.TrimSpace(c.GetHeader("X-Request-Id"))
	retryable := relayErr.Retryable
	if !retryable {
		retryable = statusCode == http.StatusTooManyRequests || statusCode == http.StatusRequestTimeout || statusCode >= http.StatusInternalServerError
	}
	errorType := strings.TrimSpace(relayErr.Type)
	if errorType == "" {
		if statusCode == http.StatusTooManyRequests {
			errorType = "rate_limit_error"
		} else if statusCode >= http.StatusInternalServerError {
			errorType = "server_error"
		} else {
			errorType = "invalid_request_error"
		}
	}
	errorCode := strings.TrimSpace(relayErr.Code)
	if errorCode == "" {
		switch {
		case statusCode == http.StatusTooManyRequests:
			errorCode = "rate_limited"
		case statusCode >= http.StatusInternalServerError:
			errorCode = "service_unavailable"
		}
	}
	c.JSON(statusCode, vos.OpenAIErrorResponse{Error: vos.OpenAIError{
		Message:           relayErr.Message,
		Type:              errorType,
		Code:              errorCode,
		Retryable:         &retryable,
		RetryAfterSeconds: retryAfterSeconds,
		RequestID:         requestID,
	}})
}

func ensureRelayRequestID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	requestID := strings.TrimSpace(c.GetHeader("X-Request-Id"))
	if requestID == "" {
		requestID = uuid.NewString()
		if c.Request != nil {
			c.Request.Header.Set("X-Request-Id", requestID)
		}
	}
	c.Header("X-Request-Id", requestID)
	return requestID
}

func retryAfterHeaderSeconds(value time.Duration) int64 {
	if value <= 0 {
		return 0
	}
	seconds := int64((value + time.Second - 1) / time.Second)
	if seconds < 1 {
		return 1
	}
	return seconds
}
