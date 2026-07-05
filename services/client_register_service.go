package services

import (
	"errors"
	"strings"

	commonDomains "github.com/wfu-work/nav-common-go-lib/domains"
	commonUtils "github.com/wfu-work/nav-common-go-lib/utils"
	"gorm.io/gorm"
)

const commonUserRoleCode = "USER"

type ClientRegisterService struct{}

var ClientRegisterServiceApp = ClientRegisterService{}

type ClientRegisterRequest struct {
	Username   string `json:"username"`
	Email      string `json:"email"`
	Password   string `json:"password"`
	Captcha    string `json:"captcha"`
	InviteCode string `json:"inviteCode"`
}

type ClientRegisterResult struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	UserGuid string `json:"userGuid"`
}

func (s ClientRegisterService) Register(req ClientRegisterRequest) (*ClientRegisterResult, error) {
	req = normalizeClientRegisterRequest(req)
	if req.Username == "" {
		return nil, errors.New("username required")
	}
	if req.Email == "" {
		return nil, errors.New("email required")
	}
	if req.Password == "" {
		return nil, errors.New("password required")
	}
	if req.Captcha == "" {
		return nil, errors.New("email code required")
	}
	settings := RegisterSettingServiceApp.Get()
	if !settings.Enabled {
		return nil, errors.New("register is disabled")
	}
	if settings.RequireInvite && req.InviteCode == "" {
		return nil, errors.New("invite code is required")
	}
	password, err := commonUtils.AesDecrypt(req.Password)
	if err != nil {
		return nil, errors.New("decrypt password failed")
	}
	hashedPassword := commonUtils.BcryptHash(password)
	var created commonDomains.SysUser
	err = MessageEmailCodeServiceApp.DB().Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&commonDomains.SysUser{}).Where("username = ?", req.Username).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("username already exists")
		}
		if err := tx.Model(&commonDomains.SysUser{}).Where("email = ?", req.Email).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("email already exists")
		}
		if err := MessageEmailCodeServiceApp.WithDB(tx).Verify(req.Email, MessageSceneRegister, req.Captcha); err != nil {
			return err
		}
		created = commonDomains.SysUser{
			Username: req.Username,
			Password: hashedPassword,
			Email:    req.Email,
			NickName: req.Username,
			Enable:   1,
		}
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
		if err := assignCommonUserRole(tx, created.Guid); err != nil {
			return err
		}
		if err := UserQuotaServiceApp.WithDB(tx).Ensure(tx, created.Guid); err != nil {
			return err
		}
		if req.InviteCode != "" {
			_, err := InvitationServiceApp.WithDB(tx).AcceptInvite(created.Guid, AcceptInviteRequest{Code: req.InviteCode})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &ClientRegisterResult{Email: created.Email, Username: created.Username, UserGuid: created.Guid}, nil
}

func assignCommonUserRole(tx *gorm.DB, userGuid string) error {
	if strings.TrimSpace(userGuid) == "" {
		return nil
	}
	var role commonDomains.SysRole
	if err := tx.Where("code = ?", commonUserRoleCode).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	return tx.Where(commonDomains.SysUserRole{UserGuid: userGuid, RoleGuid: role.Guid}).
		FirstOrCreate(&commonDomains.SysUserRole{UserGuid: userGuid, RoleGuid: role.Guid}).Error
}

func normalizeClientRegisterRequest(req ClientRegisterRequest) ClientRegisterRequest {
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Password = strings.TrimSpace(req.Password)
	req.Captcha = strings.TrimSpace(req.Captcha)
	req.InviteCode = strings.TrimSpace(req.InviteCode)
	return req
}
