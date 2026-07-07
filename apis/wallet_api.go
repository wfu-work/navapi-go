package apis

import (
	"net/http"
	"strings"

	"navapi-go/domains"
	"navapi-go/services"
	"navapi-go/vos"

	"github.com/gin-gonic/gin"
	"github.com/wfu-work/nav-common-go-lib/response"
	commonUtils "github.com/wfu-work/nav-common-go-lib/utils"
)

type WalletApi struct{}

// Self 当前用户钱包
// @Summary 当前用户钱包
// @Description 当前用户钱包
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=domains.UserWallet,msg=string}
// @Router /wallet/self [get]
func (a WalletApi) Self(c *gin.Context) {
	wallet, err := services.UserWalletServiceApp.Get(commonUtils.GetUserGuid(c))
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(wallet, c)
}

// SelfRecords 当前用户钱包流水
// @Summary 当前用户钱包流水
// @Description 当前用户钱包流水
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param size query int false "每页数量"
// @Param q query string false "关键词"
// @Success 200 {object} response.Response{data=vos.PageResult,msg=string}
// @Router /wallet/self/records [get]
func (a WalletApi) SelfRecords(c *gin.Context) {
	var query vos.PageQuery
	_ = c.ShouldBindQuery(&query)
	result, err := services.UserWalletServiceApp.ListRecords(commonUtils.GetUserGuid(c), query)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.Ok(result, c)
}

// NewAPIUserSelf NewAPI 兼容用户余额
// @Summary NewAPI 兼容用户余额
// @Description 兼容 CC Switch/NewAPI 生态的当前用户余额查询，支持 API Key 或登录 JWT
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Router /user/self [get]
func (a WalletApi) NewAPIUserSelf(c *gin.Context) {
	userGuid, token, ok := resolveWalletCompatAuth(c)
	if !ok {
		return
	}
	profile, err := services.UserWalletServiceApp.CompatProfile(userGuid, token)
	if err != nil {
		writeWalletCompatError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    profile,
	})
}

// BalanceCompat 通用余额查询
// @Summary 通用余额查询
// @Description 兼容 CC Switch 通用钱包余额查询，支持 API Key 或登录 JWT
// @Tags Navapi模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Router /user/balance [get]
func (a WalletApi) BalanceCompat(c *gin.Context) {
	userGuid, token, ok := resolveWalletCompatAuth(c)
	if !ok {
		return
	}
	balance, err := services.UserWalletServiceApp.CompatBalance(userGuid, token)
	if err != nil {
		writeWalletCompatError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "",
		"remaining": balance.Remaining,
		"balance":   balance.Balance,
		"quota":     balance.Quota,
		"used":      balance.Used,
		"total":     balance.Total,
		"unit":      balance.Unit,
		"currency":  balance.Currency,
		"data":      balance.Data,
	})
}

func resolveWalletCompatAuth(c *gin.Context) (string, *domains.ApiToken, bool) {
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	if auth == "" {
		auth = strings.TrimSpace(c.GetHeader("X-Api-Key"))
	}
	if auth == "" {
		writeWalletCompatError(c, http.StatusUnauthorized, "token is required")
		return "", nil, false
	}
	key := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	if key != "" {
		token, err := services.TokenServiceApp.ResolveForBalance(key, c.ClientIP())
		if err == nil {
			return token.UserGuid, token, true
		}
		if looksLikeJWT(key) {
			if claims, claimsErr := commonUtils.GetClaims(c); claimsErr == nil && claims != nil && strings.TrimSpace(claims.UserGuid) != "" {
				return strings.TrimSpace(claims.UserGuid), nil, true
			}
		}
		writeWalletCompatError(c, http.StatusUnauthorized, err.Error())
		return "", nil, false
	}
	writeWalletCompatError(c, http.StatusUnauthorized, "token is required")
	return "", nil, false
}

func looksLikeJWT(value string) bool {
	return strings.Count(strings.TrimSpace(value), ".") == 2
}

func writeWalletCompatError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{
		"success": false,
		"message": message,
		"data":    gin.H{},
	})
}
