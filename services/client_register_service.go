package services

import (
	"errors"
	"strings"

	"navapi-go/constants"

	commonDomains "github.com/wfu-work/nav-common-go-lib/domains"
	commonUtils "github.com/wfu-work/nav-common-go-lib/utils"
	"gorm.io/gorm"
)

const (
	commonUserRoleCode = "USER"
	commonUserRoleName = "普通用户"

	clientUsernameExistsMessage = "用户名已存在，请更换后重试"
	clientEmailExistsMessage    = "邮箱已被注册，请直接登录或更换邮箱"
)

type ClientRegisterService struct{}

var ClientRegisterServiceApp = new(ClientRegisterService)

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
	if isReservedClientUsername(req.Username) {
		return nil, errors.New("admin 为系统管理员账号，不能用于注册")
	}
	settings := RegisterSettingServiceApp.Get()
	if !settings.Enabled {
		return nil, errors.New(registerDisabledMessage)
	}
	if settings.RequireInvite && req.InviteCode == "" {
		return nil, errors.New("invite code is required")
	}
	if settings.RequireCaptcha && req.Captcha == "" {
		return nil, errors.New("email code required")
	}
	password, err := commonUtils.AesDecrypt(req.Password)
	if err != nil {
		return nil, errors.New("decrypt password failed")
	}
	hashedPassword := commonUtils.BcryptHash(password)
	var created commonDomains.SysUser
	err = MessageEmailCodeServiceApp.DB().Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&commonDomains.SysUser{}).Where("LOWER(username) = ?", strings.ToLower(req.Username)).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New(clientUsernameExistsMessage)
		}
		if err := tx.Model(&commonDomains.SysUser{}).Where("LOWER(email) = ?", req.Email).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New(clientEmailExistsMessage)
		}
		if settings.RequireCaptcha {
			if err := MessageEmailCodeServiceApp.WithDB(tx).Verify(req.Email, MessageSceneRegister, req.Captcha); err != nil {
				return err
			}
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
		if err := UserSettingsServiceApp.WithDB(tx).Ensure(tx, created.Guid); err != nil {
			return err
		}
		if err := UserWalletServiceApp.WithDB(tx).EnsureWithInitialAmount(tx, created.Guid, WholeAmountToMicros(settings.DefaultAmount)); err != nil {
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
			role = commonDomains.SysRole{
				Name:   commonUserRoleName,
				Code:   commonUserRoleCode,
				Sort:   3,
				Remark: commonUserRoleName,
			}
			if err := tx.Create(&role).Error; err != nil {
				return err
			}
		} else {
			return err
		}
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

func isReservedClientUsername(username string) bool {
	return strings.EqualFold(strings.TrimSpace(username), constants.AdminUsername)
}
