package apis

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func parseUintParam(c *gin.Context, key string) (uint, error) {
	raw := strings.TrimSpace(c.Param(key))
	if raw == "" {
		return 0, errors.New(key + " is required")
	}
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, errors.New("invalid " + key)
	}
	return uint(id), nil
}
