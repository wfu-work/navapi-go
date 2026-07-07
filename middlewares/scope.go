package middlewares

import (
	"strings"

	"navapi-go/constants"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/configs"
	commonUtils "github.com/wfu-work/nav-common-go-lib/utils"
)

const AdminUsername = constants.AdminUsername
const SuperAdminRoleCode = "SUPER_ADMIN"

func IsAdminUser(c *gin.Context) bool {
	return CurrentUsername(c) == AdminUsername || HasRoleCode(c, SuperAdminRoleCode)
}

func ScopedUserGuid(c *gin.Context) string {
	if IsAdminUser(c) {
		return ""
	}
	return CurrentUserGuid(c)
}

func CurrentUsername(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if value, exists := c.Get("userName"); exists {
		if username, ok := value.(string); ok {
			return strings.TrimSpace(username)
		}
	}
	if claims, ok := claimsFromContext(c); ok {
		return strings.TrimSpace(claims.Username)
	}
	claims, err := commonUtils.GetClaims(c)
	if err != nil || claims == nil {
		return ""
	}
	return strings.TrimSpace(claims.Username)
}

func CurrentUserGuid(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if value, exists := c.Get("userGuid"); exists {
		if userGuid, ok := value.(string); ok {
			return strings.TrimSpace(userGuid)
		}
	}
	if claims, ok := claimsFromContext(c); ok {
		return strings.TrimSpace(claims.UserGuid)
	}
	claims, err := commonUtils.GetClaims(c)
	if err != nil || claims == nil {
		return ""
	}
	return strings.TrimSpace(claims.UserGuid)
}

func HasRoleCode(c *gin.Context, target string) bool {
	target = strings.TrimSpace(target)
	if c == nil || target == "" {
		return false
	}
	if claims, ok := claimsFromContext(c); ok {
		return containsRoleCode(claims.RoleCodes, target)
	}
	claims, err := commonUtils.GetClaims(c)
	if err != nil || claims == nil {
		return false
	}
	return containsRoleCode(claims.RoleCodes, target)
}

func claimsFromContext(c *gin.Context) (*configs.CustomClaims, bool) {
	value, exists := c.Get("claims")
	if !exists {
		return nil, false
	}
	switch claims := value.(type) {
	case *configs.CustomClaims:
		if claims == nil {
			return nil, false
		}
		return claims, true
	case configs.CustomClaims:
		return &claims, true
	default:
		return nil, false
	}
}

func containsRoleCode(value string, target string) bool {
	for _, roleCode := range splitAccessValues(value) {
		if roleCode == target {
			return true
		}
	}
	return false
}

func splitAccessValues(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == ' '
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
