package apis

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"navapi-go/constants"
	"navapi-go/domains"
	"navapi-go/services"
	"navapi-go/vos"

	"github.com/gin-gonic/gin"
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
	c.JSON(http.StatusOK, buildTokenBalanceResponse(token))
}

func buildTokenBalanceResponse(token *domains.ApiToken) tokenBalanceResponse {
	used := services.AmountMicrosToCost(token.UsedAmountMicros)
	result := tokenBalanceResponse{
		IsActive:  token.Status == constants.StatusEnabled,
		Name:      token.Name,
		Used:      used,
		Unlimited: token.UnlimitedBalance,
		Unit:      "CNY",
	}
	if !token.UnlimitedBalance {
		balance := services.AmountMicrosToCost(token.BalanceAmountMicros)
		total := balance + used
		result.Balance = &balance
		result.Total = &total
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
	result, streamed, err := relayService.RelayHTTP(c, token, endpoint)
	if err != nil {
		if streamed && c.Writer.Written() {
			return
		}
		var relayErr *services.RelayHTTPError
		if errors.As(err, &relayErr) {
			openAIError(c, relayErr.StatusCode, relayErr.Message)
			return
		}
		openAIError(c, http.StatusBadGateway, err.Error())
		return
	}
	if streamed {
		return
	}
	for key, values := range result.Header {
		if key == "Content-Length" || key == "Transfer-Encoding" {
			continue
		}
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}
	contentType := result.Header.Get("Content-Type")
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
