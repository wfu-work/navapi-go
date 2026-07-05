package middlewares

import (
	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
)

func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if IsAdminUser(c) {
			c.Next()
			return
		}
		response.NoPermission("权限不足", c)
		c.Abort()
	}
}
