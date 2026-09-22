package services

import (
	"errors"
	"strings"

	"navapi-go/constants"
	"navapi-go/domains"
	"navapi-go/vos"

	commonDomains "github.com/wfu-work/nav-common-go-lib/domains"
	commonServices "github.com/wfu-work/nav-common-go-lib/services"
	"gorm.io/gorm"
)

type ClientUserService struct {
	commonServices.CrudService[commonDomains.SysUser]
}

type ClientUserListQuery struct {
	vos.PageQuery
	Content string `form:"content" json:"content"`
}

type ClientUserListItem struct {
	commonDomains.SysUser
	BalanceAmountMicros       int64  `json:"balanceAmountMicros"`
	TotalConsumedAmountMicros int64  `json:"totalConsumedAmountMicros"`
	Currency                  string `json:"currency"`
}

var ClientUserServiceApp = new(ClientUserService)

func (s *ClientUserService) WithDB(db *gorm.DB) *ClientUserService {
	cloned := *s
	cloned.CrudService = *s.CrudService.WithDB(db)
	return &cloned
}

func (s *ClientUserService) List(query ClientUserListQuery) (vos.PageResult, error) {
	query.Normalize()
	var users []commonDomains.SysUser
	var total int64
	db := s.DB()
	if db == nil {
		return vos.PageResult{}, errors.New("database is not initialized")
	}
	db = db.Model(&commonDomains.SysUser{})
	if keyword := strings.TrimSpace(firstNonEmpty(query.Q, query.Content)); keyword != "" {
		like := "%" + strings.ToLower(keyword) + "%"
		phoneLike := "%" + keyword + "%"
		// 管理端用户列表需要支持按用户名、邮箱检索；手机号、昵称和 GUID 保留为兼容搜索入口。
		db = db.Where(
			"username ILIKE ? OR email ILIKE ? OR phone ILIKE ? OR nick_name ILIKE ? OR guid ILIKE ?",
			like,
			like,
			phoneLike,
			like,
			like,
		)
	}
	if err := db.Count(&total).Error; err != nil {
		return vos.PageResult{}, err
	}
	if err := db.Order("id desc").Offset(query.Offset()).Limit(query.Size).Find(&users).Error; err != nil {
		return vos.PageResult{}, err
	}
	items, err := s.attachWallets(users)
	if err != nil {
		return vos.PageResult{}, err
	}
	return vos.PageResult{List: items, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s *ClientUserService) Delete(userGuid string) error {
	userGuid = strings.TrimSpace(userGuid)
	if userGuid == "" {
		return errors.New("user guid is required")
	}
	db := s.DB()
	if db == nil {
		return errors.New("database is not initialized")
	}

	return db.Transaction(func(tx *gorm.DB) error {
		var user commonDomains.SysUser
		if err := tx.Where("guid = ?", userGuid).First(&user).Error; err != nil {
			return err
		}
		if strings.EqualFold(user.Username, constants.AdminUsername) {
			return errors.New("the built-in admin user cannot be deleted")
		}

		if err := tx.Unscoped().Where("user_guid = ?", userGuid).Delete(&domains.ApiToken{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("user_guid = ?", userGuid).Delete(&domains.UsageLog{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("user_guid = ?", userGuid).Delete(&domains.UserSettings{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("user_guid = ?", userGuid).Delete(&domains.UserWalletRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("user_guid = ?", userGuid).Delete(&domains.UserWallet{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("user_guid = ?", userGuid).Delete(&domains.PaymentOrder{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("user_guid = ?", userGuid).Delete(&domains.UserSubscription{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("user_guid = ?", userGuid).Delete(&domains.Task{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("user_guid = ?", userGuid).Delete(&domains.CheckinRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("user_guid = ?", userGuid).Delete(&domains.QuotaDate{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("used_by = ?", userGuid).Delete(&domains.Redemption{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().
			Where("inviter_user_guid = ? OR invitee_user_guid = ?", userGuid, userGuid).
			Delete(&domains.InvitationRelation{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("owner_user_guid = ?", userGuid).Delete(&domains.InvitationCode{}).Error; err != nil {
			return err
		}
		if user.Email != "" {
			if err := tx.Unscoped().
				Where("LOWER(recipient_email) = LOWER(?)", user.Email).
				Delete(&domains.MessageSendRecord{}).Error; err != nil {
				return err
			}
			if err := tx.Unscoped().
				Where("LOWER(email) = LOWER(?)", user.Email).
				Delete(&domains.MessageEmailCode{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("user_guid = ?", userGuid).Delete(&commonDomains.SysUserRole{}).Error; err != nil {
			return err
		}
		return tx.Unscoped().Where("guid = ?", userGuid).Delete(&commonDomains.SysUser{}).Error
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (s *ClientUserService) attachWallets(users []commonDomains.SysUser) ([]ClientUserListItem, error) {
	items := make([]ClientUserListItem, 0, len(users))
	if len(users) == 0 {
		return items, nil
	}
	userGuids := make([]string, 0, len(users))
	for _, user := range users {
		items = append(items, ClientUserListItem{SysUser: user, Currency: "CNY"})
		if user.Guid != "" {
			userGuids = append(userGuids, user.Guid)
		}
	}
	if len(userGuids) == 0 {
		return items, nil
	}
	var wallets []domains.UserWallet
	// 用户列表只需要当前页的钱包聚合，批量查询避免前端或后端逐个用户追加查询。
	if err := s.DB().Where("user_guid IN ?", userGuids).Find(&wallets).Error; err != nil {
		return nil, err
	}
	walletByUser := make(map[string]domains.UserWallet, len(wallets))
	for _, wallet := range wallets {
		walletByUser[wallet.UserGuid] = wallet
	}
	for i := range items {
		wallet, ok := walletByUser[items[i].Guid]
		if !ok {
			continue
		}
		items[i].BalanceAmountMicros = wallet.BalanceAmountMicros
		items[i].TotalConsumedAmountMicros = wallet.TotalConsumedAmountMicros
		items[i].Currency = defaultString(wallet.Currency, "CNY")
	}
	return items, nil
}
