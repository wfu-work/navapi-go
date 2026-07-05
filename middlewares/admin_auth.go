package middlewares

import (
	"navapi-go/authz"

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

func IsAdminUser(c *gin.Context) bool {
	return authz.IsAdminUser(c)
}
