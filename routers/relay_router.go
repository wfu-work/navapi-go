package routers

import (
	"navapi-go/middlewares"

	"github.com/gin-gonic/gin"
)

type RelayRouter struct{}

func (r RelayRouter) InitRelayRouter(group *gin.RouterGroup) {
	r.initRelayRoutes(group, "../")
	r.initRelayRoutes(group, "")
}

func (r RelayRouter) initRelayRoutes(group *gin.RouterGroup, prefix string) {
	v1 := group.Group(prefix + "v1")
	v1.Use(middlewares.RequestBodyLimit())
	v1.Use(middlewares.TokenAuth())
	{
		v1.GET("/models", relayApi.Models)
		v1.GET("/models/:model", relayApi.Models)
		v1.POST("/completions", relayApi.OpenAICompletions)
		v1.POST("/chat/completions", relayApi.ChatCompletions)
		v1.POST("/embeddings", relayApi.OpenAIEmbeddings)
		v1.POST("/moderations", relayApi.OpenAIModerations)
		v1.POST("/rerank", relayApi.OpenAIRerank)
		v1.POST("/images/generations", relayApi.OpenAIImageGenerations)
		v1.POST("/audio/transcriptions", relayApi.OpenAIAudioTranscriptions)
		v1.POST("/audio/translations", relayApi.OpenAIAudioTranslations)
		v1.POST("/responses", relayApi.OpenAIResponses)
		v1.POST("/messages", relayApi.ClaudeMessages)
	}

	v1beta := group.Group(prefix + "v1beta")
	v1beta.Use(middlewares.RequestBodyLimit())
	v1beta.Use(middlewares.TokenAuth())
	{
		v1beta.POST("/models/*path", relayApi.GeminiModels)
	}

	mj := group.Group(prefix + "mj")
	mj.Use(middlewares.RequestBodyLimit())
	mj.Use(middlewares.TokenAuth())
	{
		mj.POST("/*path", relayApi.MidjourneyTask)
	}

	suno := group.Group(prefix + "suno")
	suno.Use(middlewares.RequestBodyLimit())
	suno.Use(middlewares.TokenAuth())
	{
		suno.POST("/*path", relayApi.SunoTask)
	}
}
