package services

import (
	"strings"
	"testing"

	"navapi-go/domains"

	commonDomains "github.com/wfu-work/nav-common-go-lib/domains"
	"github.com/wfu-work/nav-common-go-lib/global"
	commonUtils "github.com/wfu-work/nav-common-go-lib/utils"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMessageSeedDefaultsAndPreviewRegisterTemplate(t *testing.T) {
	withMessageTestDB(t)
	MessageTemplateServiceApp.SeedDefaults()

	tpl, err := MessageTemplateServiceApp.Get(TemplateCodeRegisterCaptcha)
	if err != nil {
		t.Fatal(err)
	}
	if tpl.Name != "用户注册验证码" || tpl.Channel != MessageChannelEmail {
		t.Fatalf("template = %+v, want register email template", tpl)
	}

	preview, err := EmailServiceApp.PreviewTemplate(EmailTemplatePreviewInput{
		Code:   TemplateCodeRegisterCaptcha,
		Values: map[string]string{"code": "654321", "email": "tester@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(preview.Subject, "654321") || !strings.Contains(preview.HTML, "654321") {
		t.Fatalf("preview = %+v, want rendered verification code", preview)
	}

	balanceTpl, err := MessageTemplateServiceApp.Get(TemplateCodeUserBalanceInsufficient)
	if err != nil {
		t.Fatal(err)
	}
	if balanceTpl.Name != "用户余额不足提醒" || balanceTpl.Channel != MessageChannelEmail {
		t.Fatalf("template = %+v, want balance warning email template", balanceTpl)
	}

	balancePreview, err := EmailServiceApp.PreviewTemplate(EmailTemplatePreviewInput{
		Code: TemplateCodeUserBalanceInsufficient,
		Values: map[string]string{
			"remainQuota": "8",
			"threshold":   "10",
			"quotaUnit":   "元",
			"rechargeUrl": "https://example.com/wallet",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(balancePreview.Subject, "余额不足") ||
		!strings.Contains(balancePreview.HTML, "8 元") ||
		!strings.Contains(balancePreview.HTML, "10 元") {
		t.Fatalf("preview = %+v, want rendered balance warning", balancePreview)
	}

	usageBillTpl, err := MessageTemplateServiceApp.Get(TemplateCodeUserDailyUsageBill)
	if err != nil {
		t.Fatal(err)
	}
	if usageBillTpl.Name != "普通用户每日用量账单" || usageBillTpl.Channel != MessageChannelEmail {
		t.Fatalf("template = %+v, want daily usage bill email template", usageBillTpl)
	}

	usageBillPreview, err := EmailServiceApp.PreviewTemplate(EmailTemplatePreviewInput{
		Code: TemplateCodeUserDailyUsageBill,
		Values: map[string]string{
			"billDate":     "2026-07-04",
			"requestCount": "42",
			"usageQuota":   "88",
			"remainQuota":  "912",
			"quotaUnit":    "点",
			"usageDetails": "<p>gpt-test：42 次 / 88 点</p>",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(usageBillPreview.Subject, "2026-07-04") ||
		!strings.Contains(usageBillPreview.HTML, "42 次") ||
		!strings.Contains(usageBillPreview.HTML, "88 点") {
		t.Fatalf("preview = %+v, want rendered daily usage bill", usageBillPreview)
	}

	adminUsageBillTpl, err := MessageTemplateServiceApp.Get(TemplateCodeAdminDailyUsageBill)
	if err != nil {
		t.Fatal(err)
	}
	if adminUsageBillTpl.Name != "管理员每日用量账单" || adminUsageBillTpl.Channel != MessageChannelEmail {
		t.Fatalf("template = %+v, want admin daily usage bill email template", adminUsageBillTpl)
	}

	adminUsageBillPreview, err := EmailServiceApp.PreviewTemplate(EmailTemplatePreviewInput{
		Code: TemplateCodeAdminDailyUsageBill,
		Values: map[string]string{
			"billDate":             "2026-07-04",
			"requestCount":         "168",
			"platformQuota":        "1880",
			"quotaUnit":            "点",
			"adminUserDetails":     "<p>alice@example.com：860 点</p>",
			"adminModelDetails":    "<p>gpt-test：1020 点</p>",
			"adminProviderDetails": "<p>openai-main：1880 点</p>",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(adminUsageBillPreview.Subject, "2026-07-04") ||
		!strings.Contains(adminUsageBillPreview.HTML, "1880 点") ||
		!strings.Contains(adminUsageBillPreview.HTML, "alice@example.com") {
		t.Fatalf("preview = %+v, want rendered admin daily usage bill", adminUsageBillPreview)
	}
}

func TestSendRegisterCodeStoresSendRecordAndEmailCode(t *testing.T) {
	db := withMessageTestDB(t)
	MessageTemplateServiceApp.SeedDefaults()
	if _, err := MessageEmailConfigServiceApp.Save(SaveMessageEmailConfigRequest{
		Name:       "smtp",
		Host:       "smtp.example.com",
		Port:       25,
		FromEmail:  "noreply@example.com",
		FromName:   "Nav API",
		Encryption: "none",
		IsDefault:  true,
	}); err != nil {
		t.Fatal(err)
	}
	previousSender := emailSendHTML
	emailSendHTML = func(_ EmailService, _ domains.MessageEmailConfig, recipients []string, subject string, htmlBody string) error {
		if len(recipients) != 1 || recipients[0] != "tester@example.com" {
			t.Fatalf("recipients = %+v, want tester@example.com", recipients)
		}
		if !strings.Contains(subject, "注册验证码") || !strings.Contains(htmlBody, "验证码") {
			t.Fatalf("subject/html not rendered as register code email: %q", subject)
		}
		return nil
	}
	t.Cleanup(func() { emailSendHTML = previousSender })

	result, err := EmailServiceApp.SendRegisterCode(SendRegisterCodeInput{Email: " Tester@Example.com ", ClientIP: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Successes != 1 || result.Failures != 0 || len(result.RecordGuids) != 1 {
		t.Fatalf("result = %+v, want one successful record", result)
	}
	var code domains.MessageEmailCode
	if err := db.Where("email = ? AND scene = ?", "tester@example.com", MessageSceneRegister).First(&code).Error; err != nil {
		t.Fatal(err)
	}
	if code.Status != MessageEmailCodePending || len(code.Code) != 6 || code.SendRecordGuid == "" {
		t.Fatalf("code = %+v, want pending 6 digit code linked to send record", code)
	}
	var record domains.MessageSendRecord
	if err := db.Where("guid = ?", code.SendRecordGuid).First(&record).Error; err != nil {
		t.Fatal(err)
	}
	if record.SendStatus != MessageSendStatusSuccess || record.RecipientEmail != "tester@example.com" {
		t.Fatalf("record = %+v, want successful register email record", record)
	}
}

func TestSendRegisterCodeRejectsRegisteredEmail(t *testing.T) {
	withMessageTestDB(t)
	if err := MessageEmailCodeServiceApp.DB().Create(&commonDomains.SysUser{
		BaseDataEntity: commonDomains.BaseDataEntity{Guid: "existing-user"},
		Username:       "existing",
		Email:          "Taken@Example.com",
		Password:       "hashed",
		Enable:         1,
	}).Error; err != nil {
		t.Fatal(err)
	}

	_, err := EmailServiceApp.SendRegisterCode(SendRegisterCodeInput{Email: "taken@example.com"})
	if err == nil || !strings.Contains(err.Error(), clientEmailExistsMessage) {
		t.Fatalf("err = %v, want registered email rejection", err)
	}
}

func TestSendRegisterCodeRejectsDisabledRegistration(t *testing.T) {
	db := withMessageTestDB(t)
	disableRegister(t)

	_, err := EmailServiceApp.SendRegisterCode(SendRegisterCodeInput{Email: "closed@example.com"})
	if err == nil || !strings.Contains(err.Error(), registerDisabledMessage) {
		t.Fatalf("err = %v, want register disabled rejection", err)
	}
	var count int64
	if err := db.Model(&domains.MessageEmailCode{}).Where("email = ?", "closed@example.com").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("email code count = %d, want none", count)
	}
}

func TestSendRegisterCodeRespectsCooldown(t *testing.T) {
	db := withMessageTestDB(t)
	MessageTemplateServiceApp.SeedDefaults()
	if _, err := MessageEmailConfigServiceApp.Save(SaveMessageEmailConfigRequest{
		Name:       "smtp",
		Host:       "smtp.example.com",
		Port:       25,
		FromEmail:  "noreply@example.com",
		FromName:   "Nav API",
		Encryption: "none",
		IsDefault:  true,
	}); err != nil {
		t.Fatal(err)
	}
	previousSender := emailSendHTML
	emailSendHTML = func(_ EmailService, _ domains.MessageEmailConfig, _ []string, _ string, _ string) error {
		return nil
	}
	t.Cleanup(func() { emailSendHTML = previousSender })

	if _, err := EmailServiceApp.SendRegisterCode(SendRegisterCodeInput{Email: "cooldown@example.com"}); err != nil {
		t.Fatal(err)
	}
	_, err := EmailServiceApp.SendRegisterCode(SendRegisterCodeInput{Email: "cooldown@example.com"})
	if err == nil || !strings.Contains(err.Error(), "too frequently") {
		t.Fatalf("err = %v, want cooldown rejection", err)
	}
	var count int64
	if err := db.Model(&domains.MessageEmailCode{}).Where("email = ?", "cooldown@example.com").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("code count = %d, want only first code saved", count)
	}
}

func TestDebugEmailConfigUsesSelectedConfigAndStoresRecord(t *testing.T) {
	db := withMessageTestDB(t)
	config, err := MessageEmailConfigServiceApp.Save(SaveMessageEmailConfigRequest{
		Name:       "debug smtp",
		Host:       "smtp.debug.example.com",
		Port:       25,
		FromEmail:  "debug@example.com",
		FromName:   "Debug Mail",
		Encryption: "none",
	})
	if err != nil {
		t.Fatal(err)
	}
	previousSender := emailSendHTML
	emailSendHTML = func(_ EmailService, cfg domains.MessageEmailConfig, recipients []string, subject string, htmlBody string) error {
		if cfg.Guid != config.Guid || cfg.FromEmail != "debug@example.com" {
			t.Fatalf("config = %+v, want selected debug config", cfg)
		}
		if len(recipients) != 1 || recipients[0] != "ops@example.com" {
			t.Fatalf("recipients = %+v, want ops@example.com", recipients)
		}
		if !strings.Contains(subject, "SMTP 调试") || !strings.Contains(htmlBody, "连通性测试") {
			t.Fatalf("subject/html not rendered as debug email: %q", subject)
		}
		return nil
	}
	t.Cleanup(func() { emailSendHTML = previousSender })

	result, err := EmailServiceApp.DebugEmailConfig(DebugEmailConfigInput{
		ConfigGuid:     config.Guid,
		RecipientEmail: "ops@example.com",
		Subject:        "SMTP 调试",
		Content:        "连通性测试",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Successes != 1 || result.Failures != 0 || len(result.RecordGuids) != 1 {
		t.Fatalf("result = %+v, want one successful debug record", result)
	}
	var record domains.MessageSendRecord
	if err := db.Where("guid = ?", result.RecordGuids[0]).First(&record).Error; err != nil {
		t.Fatal(err)
	}
	if record.TemplateCode != TemplateCodeEmailConfigTest || record.FromEmail != "debug@example.com" || record.SendStatus != MessageSendStatusSuccess {
		t.Fatalf("record = %+v, want successful debug send record", record)
	}
}

func TestDebugEmailConfigCanRenderSelectedTemplate(t *testing.T) {
	db := withMessageTestDB(t)
	MessageTemplateServiceApp.SeedDefaults()
	config, err := MessageEmailConfigServiceApp.Save(SaveMessageEmailConfigRequest{
		Name:       "template debug smtp",
		Host:       "smtp.template.example.com",
		Port:       25,
		FromEmail:  "template@example.com",
		FromName:   "Template Mail",
		Encryption: "none",
	})
	if err != nil {
		t.Fatal(err)
	}
	previousSender := emailSendHTML
	emailSendHTML = func(_ EmailService, cfg domains.MessageEmailConfig, recipients []string, subject string, htmlBody string) error {
		if cfg.Guid != config.Guid {
			t.Fatalf("config = %+v, want selected debug config", cfg)
		}
		if len(recipients) != 1 || recipients[0] != "admin@example.com" {
			t.Fatalf("recipients = %+v, want admin@example.com", recipients)
		}
		if !strings.Contains(subject, "2026-07-04") || !strings.Contains(htmlBody, "1880 点") {
			t.Fatalf("subject/html not rendered from selected template: %q", subject)
		}
		return nil
	}
	t.Cleanup(func() { emailSendHTML = previousSender })

	result, err := EmailServiceApp.DebugEmailConfig(DebugEmailConfigInput{
		ConfigGuid:     config.Guid,
		RecipientEmail: "admin@example.com",
		TemplateCode:   TemplateCodeAdminDailyUsageBill,
		Values: map[string]string{
			"billDate":      "2026-07-04",
			"platformQuota": "1880",
			"quotaUnit":     "点",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Successes != 1 || result.Failures != 0 || len(result.RecordGuids) != 1 {
		t.Fatalf("result = %+v, want one successful template debug record", result)
	}
	var record domains.MessageSendRecord
	if err := db.Where("guid = ?", result.RecordGuids[0]).First(&record).Error; err != nil {
		t.Fatal(err)
	}
	if record.TemplateCode != TemplateCodeAdminDailyUsageBill || record.TemplateName != "管理员每日用量账单" || record.SendStatus != MessageSendStatusSuccess {
		t.Fatalf("record = %+v, want selected template send record", record)
	}
}

func TestClientRegisterConsumesEmailCodeAndCreatesUser(t *testing.T) {
	db := withMessageTestDB(t)
	password, err := commonUtils.AesEncrypt("secret123")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := MessageEmailCodeServiceApp.Save(domains.MessageEmailCode{
		Email:       "new@example.com",
		Scene:       MessageSceneRegister,
		Code:        "123456",
		Status:      MessageEmailCodePending,
		ExpiresTime: nowMilli() + 60000,
	}); err != nil {
		t.Fatal(err)
	}

	result, err := ClientRegisterServiceApp.Register(ClientRegisterRequest{
		Username: "new-user",
		Email:    "new@example.com",
		Password: password,
		Captcha:  "123456",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.UserGuid == "" || result.Username != "new-user" {
		t.Fatalf("result = %+v, want created user", result)
	}
	var user commonDomains.SysUser
	if err := db.Where("guid = ?", result.UserGuid).First(&user).Error; err != nil {
		t.Fatal(err)
	}
	if user.Email != "new@example.com" || user.Password == "" {
		t.Fatalf("user = %+v, want registered email and hashed password", user)
	}
	var code domains.MessageEmailCode
	if err := db.Where("email = ?", "new@example.com").First(&code).Error; err != nil {
		t.Fatal(err)
	}
	if code.Status != MessageEmailCodeUsed || code.UsedTime == 0 {
		t.Fatalf("code = %+v, want used email code", code)
	}
	var quota domains.UserQuota
	if err := db.Where("user_guid = ?", result.UserGuid).First(&quota).Error; err != nil {
		t.Fatal(err)
	}
	if quota.RemainQuota != defaultRegisterQuota || quota.TotalQuota != defaultRegisterQuota {
		t.Fatalf("quota = %+v, want default register quota", quota)
	}
	var settings domains.UserSettings
	if err := db.Where("user_guid = ?", result.UserGuid).First(&settings).Error; err != nil {
		t.Fatal(err)
	}
	if !settings.QuotaReminderEnabled ||
		!settings.PlatformAnnouncementEnabled ||
		settings.AbnormalCallAlertEnabled ||
		settings.MaxConcurrency != DefaultUserMaxConcurrency ||
		settings.ExtraConfig != "{}" {
		t.Fatalf("settings = %+v, want default user settings", settings)
	}
	var wallet domains.UserWallet
	if err := db.Where("user_guid = ?", result.UserGuid).First(&wallet).Error; err != nil {
		t.Fatal(err)
	}
	var userRole commonDomains.SysUserRole
	if err := db.Where("user_guid = ?", result.UserGuid).First(&userRole).Error; err != nil {
		t.Fatal(err)
	}
	var role commonDomains.SysRole
	if err := db.Where("guid = ?", userRole.RoleGuid).First(&role).Error; err != nil {
		t.Fatal(err)
	}
	if role.Code != commonUserRoleCode {
		t.Fatalf("role = %+v, want common user role", role)
	}
}

func TestClientRegisterRejectsAdminUsername(t *testing.T) {
	db := withMessageTestDB(t)
	_, err := ClientRegisterServiceApp.Register(ClientRegisterRequest{
		Username: " Admin ",
		Email:    "admin-new@example.com",
		Password: "not-used",
		Captcha:  "123456",
	})
	if err == nil || !strings.Contains(err.Error(), "系统管理员账号") {
		t.Fatalf("err = %v, want reserved admin username rejection", err)
	}
	var count int64
	if err := db.Model(&commonDomains.SysUser{}).Where("LOWER(username) = ?", "admin").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("admin users = %d, want none", count)
	}
}

func TestClientRegisterRejectsDuplicateUsernameAndEmail(t *testing.T) {
	db := withMessageTestDB(t)
	if err := db.Create(&commonDomains.SysUser{
		BaseDataEntity: commonDomains.BaseDataEntity{Guid: "existing-unique-user"},
		Username:       "ExistingUser",
		Email:          "Taken@Example.com",
		Password:       "hashed",
		Enable:         1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	password, err := commonUtils.AesEncrypt("secret123")
	if err != nil {
		t.Fatal(err)
	}

	_, err = ClientRegisterServiceApp.Register(ClientRegisterRequest{
		Username: " existinguser ",
		Email:    "new-unique@example.com",
		Password: password,
		Captcha:  "123456",
	})
	if err == nil || !strings.Contains(err.Error(), clientUsernameExistsMessage) {
		t.Fatalf("err = %v, want duplicate username rejection", err)
	}

	_, err = ClientRegisterServiceApp.Register(ClientRegisterRequest{
		Username: "new-unique-user",
		Email:    "taken@example.com",
		Password: password,
		Captcha:  "123456",
	})
	if err == nil || !strings.Contains(err.Error(), clientEmailExistsMessage) {
		t.Fatalf("err = %v, want duplicate email rejection", err)
	}

	var count int64
	if err := db.Model(&commonDomains.SysUser{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("user count = %d, want only existing user", count)
	}
}

func TestClientRegisterRejectsDisabledRegistration(t *testing.T) {
	db := withMessageTestDB(t)
	disableRegister(t)

	_, err := ClientRegisterServiceApp.Register(ClientRegisterRequest{
		Username: "closed-user",
		Email:    "closed-user@example.com",
		Password: "not-used",
		Captcha:  "123456",
	})
	if err == nil || !strings.Contains(err.Error(), registerDisabledMessage) {
		t.Fatalf("err = %v, want register disabled rejection", err)
	}
	var count int64
	if err := db.Model(&commonDomains.SysUser{}).Where("email = ?", "closed-user@example.com").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("user count = %d, want none", count)
	}
}

func TestClientRegisterCreatesCommonUserRoleWhenMissing(t *testing.T) {
	db := withMessageTestDB(t)
	if err := db.Where("code = ?", commonUserRoleCode).Delete(&commonDomains.SysRole{}).Error; err != nil {
		t.Fatal(err)
	}
	password, err := commonUtils.AesEncrypt("secret123")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := MessageEmailCodeServiceApp.Save(domains.MessageEmailCode{
		Email:       "role-new@example.com",
		Scene:       MessageSceneRegister,
		Code:        "654321",
		Status:      MessageEmailCodePending,
		ExpiresTime: nowMilli() + 60000,
	}); err != nil {
		t.Fatal(err)
	}

	result, err := ClientRegisterServiceApp.Register(ClientRegisterRequest{
		Username: "role-new-user",
		Email:    "role-new@example.com",
		Password: password,
		Captcha:  "654321",
	})
	if err != nil {
		t.Fatal(err)
	}
	var role commonDomains.SysRole
	if err := db.Where("code = ?", commonUserRoleCode).First(&role).Error; err != nil {
		t.Fatal(err)
	}
	var userRole commonDomains.SysUserRole
	if err := db.Where("user_guid = ? AND role_guid = ?", result.UserGuid, role.Guid).First(&userRole).Error; err != nil {
		t.Fatal(err)
	}
}

func withMessageTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := global.NAV_DB
	previousCache := OptionServiceApp.cache
	RateLimitServiceApp.mu.Lock()
	previousRateLimitBuckets := RateLimitServiceApp.buckets
	RateLimitServiceApp.buckets = map[string]*rateLimitBucket{}
	RateLimitServiceApp.mu.Unlock()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&domains.MessageEmailConfig{},
		&domains.MessageTemplate{},
		&domains.MessageSendRecord{},
		&domains.MessageEmailCode{},
		&domains.UserQuota{},
		&domains.UserWallet{},
		&domains.UserWalletRecord{},
		&domains.UserSettings{},
		&domains.Option{},
		&domains.Setting{},
		&commonDomains.SysUser{},
		&commonDomains.SysRole{},
		&commonDomains.SysUserRole{},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&commonDomains.SysRole{
		BaseDataEntity: commonDomains.BaseDataEntity{Guid: "role-user"},
		Name:           "普通用户",
		Code:           commonUserRoleCode,
		Sort:           3,
	}).Error; err != nil {
		t.Fatal(err)
	}
	global.NAV_DB = db
	OptionServiceApp.cache = map[string]string{}
	t.Cleanup(func() {
		global.NAV_DB = previousDB
		OptionServiceApp.cache = previousCache
		RateLimitServiceApp.mu.Lock()
		RateLimitServiceApp.buckets = previousRateLimitBuckets
		RateLimitServiceApp.mu.Unlock()
	})
	return db
}

func disableRegister(t *testing.T) {
	t.Helper()
	if err := RegisterSettingServiceApp.Set(RegisterSettings{
		Enabled:        false,
		DefaultQuota:   defaultRegisterQuota,
		DefaultGroup:   "default",
		RequireCaptcha: true,
	}); err != nil {
		t.Fatal(err)
	}
}
