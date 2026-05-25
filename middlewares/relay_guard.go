package middlewares

import (
	"net/http"

	"navapi-go/services"

	"github.com/gin-gonic/gin"
)

const defaultMaxRequestBodyBytes int64 = 32 << 20

func RequestBodyLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := services.OptionServiceApp.Int64("relay.max_body_bytes", defaultMaxRequestBodyBytes)
		if limit <= 0 {
			c.Next()
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}
