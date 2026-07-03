package apis

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"navapi-go/constants"
	"navapi-go/domains"
	"navapi-go/dto"
	"navapi-go/services"

	"github.com/gin-gonic/gin"
)

type RelayApi struct{}

func (a RelayApi) Models(c *gin.Context) {
	models, err := services.ModelServiceApp.ListOpenAIModels()
	if err != nil {
		openAIError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if token, ok := c.Get(constants.ContextToken); ok {
		if apiToken, ok := token.(*domains.ApiToken); ok && apiToken != nil {
			filtered := models.Data[:0]
			for _, model := range models.Data {
				if services.TokenServiceApp.CheckModel(apiToken, model.ID) == nil {
					filtered = append(filtered, model)
				}
			}
			models.Data = filtered
		}
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
	result, streamed, err := services.RelayServiceApp.RelayHTTP(c, token, endpoint)
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
	if err := services.TaskServiceApp.Create(&task); err != nil {
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
	c.JSON(code, dto.OpenAIErrorResponse{Error: dto.OpenAIError{
		Message: message,
		Type:    "invalid_request_error",
	}})
}
