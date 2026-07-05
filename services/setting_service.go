package services

import (
	"errors"
	"strings"

	"navapi-go/domains"

	commonServices "github.com/wfu-work/nav-common-go-lib/services"
	commonUtils "github.com/wfu-work/nav-common-go-lib/utils"
	"gorm.io/gorm/clause"
)

type SettingService struct {
	commonServices.CrudService[domains.Setting]
}

var SettingServiceApp = new(SettingService)

const (
	settingContactQQGroupNo     = "contact.qq.group_no"
	settingContactQQGroupQRCode = "contact.qq.group_qr_code"
	settingContactWechatAccount = "contact.wechat.account"
	settingContactWechatQRCode  = "contact.wechat.qr_code"
	settingContactSponsorQRCode = "contact.sponsor.qr_code"
)

type ContactSettings struct {
	QQGroupNo     string `json:"qqGroupNo"`
	QQGroupQRCode string `json:"qqGroupQrCode"`
	WechatAccount string `json:"wechatAccount"`
	WechatQRCode  string `json:"wechatQrCode"`
	SponsorQRCode string `json:"sponsorQrCode"`
}

// List returns paginated runtime settings.
func (s SettingService) List(params map[string]string) (interface{}, int64, error) {
	return s.CrudService.List(commonUtils.ToPageInfo(params), "key,description")
}

// Save creates or updates a runtime setting by key.
func (s SettingService) Save(setting domains.Setting) error {
	setting.Key = strings.TrimSpace(setting.Key)
	if setting.Key == "" {
		return errors.New("missing setting key")
	}
	if hook, ok := any(&setting).(beforeCreateHook); ok && (setting.Guid == "" || setting.UpdateTime == 0) {
		if err := hook.BeforeCreate(nil); err != nil {
			return err
		}
	}
	return s.DB().Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "key"}},
		DoUpdates: clause.Assignments(map[string]any{
			"value":       setting.Value,
			"description": setting.Description,
			"update_time": setting.UpdateTime,
		}),
	}).Create(&setting).Error
}

// Delete soft-deletes one setting by guid.
func (s SettingService) Delete(guid string) error {
	if guid == "" {
		return errors.New("missing setting guid")
	}
	return s.CrudService.DeleteByGuid(guid)
}

func (s SettingService) ContactSettings() (ContactSettings, error) {
	settings := ContactSettings{}
	keys := []string{
		settingContactQQGroupNo,
		settingContactQQGroupQRCode,
		settingContactWechatAccount,
		settingContactWechatQRCode,
		settingContactSponsorQRCode,
	}
	var rows []domains.Setting
	if err := s.DB().Where("key IN ?", keys).Find(&rows).Error; err != nil {
		return settings, err
	}
	values := map[string]string{}
	for _, row := range rows {
		values[row.Key] = row.Value
	}
	settings.QQGroupNo = values[settingContactQQGroupNo]
	settings.QQGroupQRCode = values[settingContactQQGroupQRCode]
	settings.WechatAccount = values[settingContactWechatAccount]
	settings.WechatQRCode = values[settingContactWechatQRCode]
	settings.SponsorQRCode = values[settingContactSponsorQRCode]
	return settings, nil
}

func (s SettingService) SaveContactSettings(settings ContactSettings) (ContactSettings, error) {
	settings = normalizeContactSettings(settings)
	items := []domains.Setting{
		{Key: settingContactQQGroupNo, Value: settings.QQGroupNo, Description: "联系配置：QQ群号"},
		{Key: settingContactQQGroupQRCode, Value: settings.QQGroupQRCode, Description: "联系配置：QQ群二维码图片"},
		{Key: settingContactWechatAccount, Value: settings.WechatAccount, Description: "联系配置：微信号"},
		{Key: settingContactWechatQRCode, Value: settings.WechatQRCode, Description: "联系配置：微信二维码图片"},
		{Key: settingContactSponsorQRCode, Value: settings.SponsorQRCode, Description: "联系配置：赞助收款码图片"},
	}
	for _, item := range items {
		if err := s.Save(item); err != nil {
			return ContactSettings{}, err
		}
	}
	return s.ContactSettings()
}

func normalizeContactSettings(settings ContactSettings) ContactSettings {
	settings.QQGroupNo = strings.TrimSpace(settings.QQGroupNo)
	settings.QQGroupQRCode = strings.TrimSpace(settings.QQGroupQRCode)
	settings.WechatAccount = strings.TrimSpace(settings.WechatAccount)
	settings.WechatQRCode = strings.TrimSpace(settings.WechatQRCode)
	settings.SponsorQRCode = strings.TrimSpace(settings.SponsorQRCode)
	return settings
}
