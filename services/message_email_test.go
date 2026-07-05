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
}

func withMessageTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := global.NAV_DB
	previousCache := OptionServiceApp.cache
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
		&domains.Option{},
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
	})
	return db
}
