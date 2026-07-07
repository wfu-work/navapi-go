package services

import (
	"errors"
	"strings"

	"navapi-go/domains"

	commonDomains "github.com/wfu-work/nav-common-go-lib/domains"
)

const WalletCompatUnit = "积分"

type WalletCompatProfile struct {
	ID                     uint   `json:"id,omitempty"`
	UserGuid               string `json:"user_guid"`
	Username               string `json:"username,omitempty"`
	Email                  string `json:"email,omitempty"`
	Quota                  int64  `json:"quota"`
	RemainingQuota         int64  `json:"remaining_quota"`
	UsedQuota              int64  `json:"used_quota"`
	TotalQuota             int64  `json:"total_quota"`
	Group                  string `json:"group,omitempty"`
	AllowedGroups          string `json:"allowed_groups,omitempty"`
	Balance                int64  `json:"balance"`
	Unit                   string `json:"unit"`
	Currency               string `json:"currency,omitempty"`
	PaidBalanceQuota       int64  `json:"paid_balance_quota"`
	RewardBalanceQuota     int64  `json:"reward_balance_quota"`
	CommissionBalanceQuota int64  `json:"commission_balance_quota"`
	TokenID                uint   `json:"token_id,omitempty"`
	TokenGuid              string `json:"token_guid,omitempty"`
	TokenName              string `json:"token_name,omitempty"`
	TokenQuota             int64  `json:"token_quota,omitempty"`
	TokenUsedQuota         int64  `json:"token_used_quota,omitempty"`
	TokenUnlimitedQuota    bool   `json:"token_unlimited_quota,omitempty"`
}

type WalletCompatBalance struct {
	Remaining int64                `json:"remaining"`
	Balance   int64                `json:"balance"`
	Quota     int64                `json:"quota"`
	Used      int64                `json:"used"`
	Total     int64                `json:"total"`
	Unit      string               `json:"unit"`
	Currency  string               `json:"currency,omitempty"`
	Data      *WalletCompatProfile `json:"data"`
}

func (s *UserWalletService) CompatProfile(userGuid string, token *domains.ApiToken) (*WalletCompatProfile, error) {
	userGuid = strings.TrimSpace(userGuid)
	if userGuid == "" && token != nil {
		userGuid = strings.TrimSpace(token.UserGuid)
	}
	if userGuid == "" {
		return nil, errors.New("user guid is required")
	}
	wallet, err := s.Get(userGuid)
	if err != nil {
		return nil, err
	}
	quota, _ := UserQuotaServiceApp.Get(userGuid)
	totalQuota := walletTotalQuota(wallet)
	profile := &WalletCompatProfile{
		UserGuid:               userGuid,
		Quota:                  wallet.BalanceQuota,
		RemainingQuota:         wallet.BalanceQuota,
		UsedQuota:              wallet.TotalConsumedQuota,
		TotalQuota:             totalQuota,
		Balance:                wallet.BalanceQuota,
		Unit:                   WalletCompatUnit,
		Currency:               wallet.Currency,
		PaidBalanceQuota:       wallet.PaidBalanceQuota,
		RewardBalanceQuota:     wallet.RewardBalanceQuota,
		CommissionBalanceQuota: wallet.CommissionBalanceQuota,
	}
	if quota != nil {
		profile.Group = quota.Group
		profile.AllowedGroups = quota.AllowedGroups
	}
	if token != nil {
		profile.TokenID = token.Id
		profile.TokenGuid = token.Guid
		profile.TokenName = token.Name
		profile.TokenQuota = token.RemainQuota
		profile.TokenUsedQuota = token.UsedQuota
		profile.TokenUnlimitedQuota = token.UnlimitedQuota
	}
	fillCompatUser(profile)
	return profile, nil
}

func (s *UserWalletService) CompatBalance(userGuid string, token *domains.ApiToken) (*WalletCompatBalance, error) {
	profile, err := s.CompatProfile(userGuid, token)
	if err != nil {
		return nil, err
	}
	return &WalletCompatBalance{
		Remaining: profile.RemainingQuota,
		Balance:   profile.Balance,
		Quota:     profile.Quota,
		Used:      profile.UsedQuota,
		Total:     profile.TotalQuota,
		Unit:      profile.Unit,
		Currency:  profile.Currency,
		Data:      profile,
	}, nil
}

func walletTotalQuota(wallet *domains.UserWallet) int64 {
	if wallet == nil {
		return 0
	}
	incomeTotal := wallet.TotalRechargeQuota + wallet.TotalSubscriptionQuota + wallet.TotalRewardQuota + wallet.TotalCommissionQuota
	runningTotal := wallet.BalanceQuota + wallet.TotalConsumedQuota
	if incomeTotal > runningTotal {
		return incomeTotal
	}
	return runningTotal
}

func fillCompatUser(profile *WalletCompatProfile) {
	if profile == nil || strings.TrimSpace(profile.UserGuid) == "" {
		return
	}
	var user commonDomains.SysUser
	if err := UserWalletServiceApp.DB().Where("guid = ?", profile.UserGuid).First(&user).Error; err != nil {
		return
	}
	profile.ID = user.Id
	profile.Username = user.Username
	profile.Email = user.Email
}
