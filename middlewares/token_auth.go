package middlewares

import (
	"net/http"
	"strings"

	"navapi-go/constants"
	"navapi-go/dto"
	"navapi-go/services"

	"github.com/gin-gonic/gin"
)

func TokenAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" {
			auth = c.GetHeader("X-Api-Key")
		}
		tokenValue := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		token, err := services.TokenServiceApp.Validate(tokenValue, c.ClientIP())
		if err != nil {
			c.JSON(http.StatusUnauthorized, dto.OpenAIErrorResponse{Error: dto.OpenAIError{
				Message: err.Error(),
				Type:    "invalid_request_error",
				Code:    "invalid_api_key",
			}})
			c.Abort()
			return
		}
		c.Set(constants.ContextToken, token)
		c.Next()
	}
}
