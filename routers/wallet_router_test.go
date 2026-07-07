package routers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"navapi-go/constants"
	"navapi-go/domains"

	"github.com/gin-gonic/gin"
	commonDomains "github.com/wfu-work/nav-common-go-lib/domains"
	"github.com/wfu-work/nav-common-go-lib/global"
	commonRouters "github.com/wfu-work/nav-common-go-lib/routers"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestWalletCompatRoutesReturnBalanceForAPIKey(t *testing.T) {
	previousDB := global.NAV_DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&domains.ApiToken{},
		&domains.UserQuota{},
		&domains.UserWallet{},
		&domains.UserWalletRecord{},
		&commonDomains.SysUser{},
	); err != nil {
		t.Fatal(err)
	}
	global.NAV_DB = db
	t.Cleanup(func() {
		global.NAV_DB = previousDB
	})

	if err := db.Create(&commonDomains.SysUser{
		BaseDataEntity: commonDomains.BaseDataEntity{Guid: "user-wallet"},
		Username:       "wallet-user",
		Email:          "wallet@example.com",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&domains.UserQuota{UserGuid: "user-wallet", RemainQuota: 120, UsedQuota: 30, TotalQuota: 150, Group: "vip"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&domains.UserWallet{UserGuid: "user-wallet", BalanceQuota: 120, PaidBalanceQuota: 120, TotalConsumedQuota: 30, TotalRechargeQuota: 150, Currency: "CNY"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&domains.ApiToken{
		UserGuid:    "user-wallet",
		Name:        "empty token",
		Key:         "sk-wallet-balance",
		Status:      constants.StatusEnabled,
		Group:       "vip",
		RemainQuota: 0,
		UsedQuota:   30,
		ExpiredTime: -1,
	}).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	publicGroup := engine.Group("/api")
	privateGroup := engine.Group("/api")
	commonRouters.SysRouterGroupApp.InitUserRouter(privateGroup)
	RouterGroupApp.InitRouters(publicGroup, privateGroup)

	selfRecorder := httptest.NewRecorder()
	selfReq := httptest.NewRequest(http.MethodGet, "/api/user/self", nil)
	selfReq.Header.Set("Authorization", "Bearer sk-wallet-balance")
	engine.ServeHTTP(selfRecorder, selfReq)
	if selfRecorder.Code != http.StatusOK {
		t.Fatalf("/api/user/self status = %d body = %s", selfRecorder.Code, selfRecorder.Body.String())
	}
	var selfBody struct {
		Success bool `json:"success"`
		Data    struct {
			UserGuid       string `json:"user_guid"`
			Username       string `json:"username"`
			Quota          int64  `json:"quota"`
			UsedQuota      int64  `json:"used_quota"`
			TotalQuota     int64  `json:"total_quota"`
			Group          string `json:"group"`
			TokenQuota     int64  `json:"token_quota"`
			TokenUsedQuota int64  `json:"token_used_quota"`
			Unit           string `json:"unit"`
		} `json:"data"`
	}
	if err := json.Unmarshal(selfRecorder.Body.Bytes(), &selfBody); err != nil {
		t.Fatal(err)
	}
	if !selfBody.Success ||
		selfBody.Data.UserGuid != "user-wallet" ||
		selfBody.Data.Username != "wallet-user" ||
		selfBody.Data.Quota != 120 ||
		selfBody.Data.UsedQuota != 30 ||
		selfBody.Data.TotalQuota != 150 ||
		selfBody.Data.Group != "vip" ||
		selfBody.Data.TokenQuota != 0 ||
		selfBody.Data.TokenUsedQuota != 30 ||
		selfBody.Data.Unit != "积分" {
		t.Fatalf("self body = %+v, want NewAPI-compatible wallet data", selfBody)
	}

	balanceRecorder := httptest.NewRecorder()
	balanceReq := httptest.NewRequest(http.MethodGet, "/user/balance", nil)
	balanceReq.Header.Set("X-Api-Key", "sk-wallet-balance")
	engine.ServeHTTP(balanceRecorder, balanceReq)
	if balanceRecorder.Code != http.StatusOK {
		t.Fatalf("/user/balance status = %d body = %s", balanceRecorder.Code, balanceRecorder.Body.String())
	}
	var balanceBody struct {
		Success   bool   `json:"success"`
		Remaining int64  `json:"remaining"`
		Balance   int64  `json:"balance"`
		Quota     int64  `json:"quota"`
		Used      int64  `json:"used"`
		Total     int64  `json:"total"`
		Unit      string `json:"unit"`
	}
	if err := json.Unmarshal(balanceRecorder.Body.Bytes(), &balanceBody); err != nil {
		t.Fatal(err)
	}
	if !balanceBody.Success ||
		balanceBody.Remaining != 120 ||
		balanceBody.Balance != 120 ||
		balanceBody.Quota != 120 ||
		balanceBody.Used != 30 ||
		balanceBody.Total != 150 ||
		balanceBody.Unit != "积分" {
		t.Fatalf("balance body = %+v, want generic wallet balance data", balanceBody)
	}
}

func TestWalletCompatRouteRequiresToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	publicGroup := engine.Group("/api")
	privateGroup := engine.Group("/api")
	RouterGroupApp.InitRouters(publicGroup, privateGroup)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/user/self", nil)
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body = %s, want 401", recorder.Code, recorder.Body.String())
	}
}
