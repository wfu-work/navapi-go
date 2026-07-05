package middlewares

import (
	"net/http"
	"strings"

	"navapi-go/constants"
	"navapi-go/services"
	"navapi-go/vos"

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
			c.JSON(http.StatusUnauthorized, vos.OpenAIErrorResponse{Error: vos.OpenAIError{
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
