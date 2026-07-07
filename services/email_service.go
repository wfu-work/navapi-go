package services

import (
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"math/big"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"navapi-go/domains"
	"navapi-go/utils"

	"github.com/google/uuid"
	commonDomains "github.com/wfu-work/nav-common-go-lib/domains"
)

const emailDialTimeout = 10 * time.Second

type EmailService struct{}

var EmailServiceApp = EmailService{}

type EmailTemplateInput struct {
	Code      string
	Title     string
	Variables map[string]string
	To        []string
}

type EmailTemplatePreviewInput struct {
	Code    string            `json:"code"`
	Subject string            `json:"subject"`
	Content string            `json:"content"`
	Values  map[string]string `json:"values"`
}

type EmailTemplatePreviewResult struct {
	Subject string `json:"subject"`
	HTML    string `json:"html"`
}

type EmailSendResult struct {
	Recipients     int                `json:"recipients"`
	Subject        string             `json:"subject"`
	Successes      int                `json:"successes"`
	Failures       int                `json:"failures"`
	BatchGuid      string             `json:"batchGuid"`
	RecordGuids    []string           `json:"recordGuids"`
	FailureDetails []EmailSendFailure `json:"failureDetails,omitempty"`
}

type EmailSendFailure struct {
	RecordGuid     string `json:"recordGuid,omitempty"`
	RecipientEmail string `json:"recipientEmail,omitempty"`
	Error          string `json:"error"`
}

type SendRegisterCodeInput struct {
	Email    string
	ClientIP string
}

type DebugEmailConfigInput struct {
	ConfigGuid     string            `json:"-"`
	RecipientEmail string            `json:"recipientEmail"`
	TemplateCode   string            `json:"templateCode"`
	Subject        string            `json:"subject"`
	Content        string            `json:"content"`
	Values         map[string]string `json:"values"`
}

var emailSendHTML = func(s EmailService, config domains.MessageEmailConfig, recipients []string, subject string, htmlBody string) error {
	return s.sendHTML(config, recipients, subject, htmlBody)
}

func (s EmailService) SendTemplate(input EmailTemplateInput) (*EmailSendResult, error) {
	code := strings.TrimSpace(input.Code)
	if code == "" {
		return nil, errors.New("template code required")
	}
	tpl, err := MessageTemplateServiceApp.GetEnabledEmail(code)
	if err != nil {
		return nil, err
	}
	recipients := normalizeEmailAddresses(input.To)
	if len(recipients) == 0 {
		return nil, errors.New("email recipients required")
	}
	subject, htmlBody := s.renderTemplate(tpl, input.Title, input.Variables)
	return s.sendRenderedHTML(emailRenderedInput{
		TemplateCode: tpl.Code,
		TemplateName: tpl.Name,
		Subject:      subject,
		HTML:         htmlBody,
		Recipients:   recipients,
	})
}

func (s EmailService) PreviewTemplate(input EmailTemplatePreviewInput) (*EmailTemplatePreviewResult, error) {
	code := strings.TrimSpace(input.Code)
	if code == "" {
		code = TemplateCodeRegisterCaptcha
	}
	subjectTemplate := strings.TrimSpace(input.Subject)
	contentTemplate := strings.TrimSpace(input.Content)
	title := "邮件预览"
	if subjectTemplate == "" || contentTemplate == "" {
		tpl, err := MessageTemplateServiceApp.Get(code)
		if err != nil {
			return nil, err
		}
		if subjectTemplate == "" {
			subjectTemplate = tpl.Subject
		}
		if contentTemplate == "" {
			contentTemplate = tpl.Content
		}
		title = tpl.Name
	}
	variables := defaultEmailTemplateVariables(code)
	for key, value := range input.Values {
		variables[key] = value
	}
	subject := strings.TrimSpace(utils.RenderTemplateText(subjectTemplate, variables))
	if subject == "" {
		subject = title
	}
	htmlBody := utils.DefaultEmailHTML(utils.EmailHTMLInput{
		Title:   title,
		Subject: subject,
		Content: utils.RenderTemplateText(contentTemplate, variables),
	})
	return &EmailTemplatePreviewResult{Subject: subject, HTML: htmlBody}, nil
}

func (s EmailService) SendRegisterCode(input SendRegisterCodeInput) (*EmailSendResult, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email == "" {
		return nil, errors.New("email required")
	}
	if !isValidEmailAddress(email) {
		return nil, errors.New("email invalid")
	}
	settings := RegisterSettingServiceApp.Get()
	if !settings.Enabled {
		return nil, errors.New(registerDisabledMessage)
	}
	if exists, err := clientEmailExists(email); err != nil {
		return nil, err
	} else if exists {
		return nil, errors.New(clientEmailExistsMessage)
	}
	if recent, err := MessageEmailCodeServiceApp.HasRecentPending(email, MessageSceneRegister, nowMilli()-int64(RegisterEmailCodeCooldown/time.Millisecond)); err != nil {
		return nil, err
	} else if recent {
		return nil, fmt.Errorf("email code sent too frequently, retry after %s", RegisterEmailCodeCooldown.Round(time.Second))
	}
	if ok, retryAfter := RateLimitServiceApp.Allow("register-email:"+email, RegisterEmailCodeHourlyLimit, time.Hour); !ok {
		return nil, fmt.Errorf("too many email code requests, retry after %s", retryAfter.Round(time.Second))
	}
	clientIP := strings.TrimSpace(input.ClientIP)
	if clientIP != "" {
		if ok, retryAfter := RateLimitServiceApp.Allow("register-ip:"+clientIP, RegisterEmailCodeHourlyLimit*3, time.Hour); !ok {
			return nil, fmt.Errorf("too many email code requests, retry after %s", retryAfter.Round(time.Second))
		}
	}
	if err := MessageEmailCodeServiceApp.ExpireOld(); err != nil {
		return nil, err
	}
	code, err := randomNumericCode(6)
	if err != nil {
		return nil, err
	}
	ttlMinutes := int(RegisterEmailCodeTTL.Minutes())
	variables := map[string]string{
		"appName":    "Nav API",
		"email":      email,
		"code":       code,
		"ttlMinutes": strconv.Itoa(ttlMinutes),
		"time":       time.Now().Format("2006-01-02 15:04:05"),
	}
	result, err := s.SendTemplate(EmailTemplateInput{
		Code:      TemplateCodeRegisterCaptcha,
		Title:     "用户注册验证码",
		Variables: variables,
		To:        []string{email},
	})
	if err != nil {
		return result, err
	}
	sendRecordGuid := ""
	if len(result.RecordGuids) > 0 {
		sendRecordGuid = result.RecordGuids[0]
	}
	_, err = MessageEmailCodeServiceApp.Save(domains.MessageEmailCode{
		Email:          email,
		Scene:          MessageSceneRegister,
		Code:           code,
		Status:         MessageEmailCodePending,
		ExpiresTime:    nowMilli() + int64(RegisterEmailCodeTTL/time.Millisecond),
		SendRecordGuid: sendRecordGuid,
		ClientIP:       clientIP,
	})
	if err != nil {
		return result, err
	}
	return result, nil
}

func isValidEmailAddress(email string) bool {
	addr, err := mail.ParseAddress(email)
	return err == nil && strings.EqualFold(addr.Address, email)
}

func clientEmailExists(email string) (bool, error) {
	var count int64
	err := MessageEmailCodeServiceApp.DB().Model(&commonDomains.SysUser{}).
		Where("LOWER(email) = ?", strings.ToLower(strings.TrimSpace(email))).
		Count(&count).Error
	return count > 0, err
}

func (s EmailService) DebugEmailConfig(input DebugEmailConfigInput) (*EmailSendResult, error) {
	guid := strings.TrimSpace(input.ConfigGuid)
	if guid == "" {
		return nil, errors.New("email config guid required")
	}
	var config domains.MessageEmailConfig
	if err := MessageEmailConfigServiceApp.DB().Where("guid = ?", guid).First(&config).Error; err != nil {
		return nil, errors.New("email config not found")
	}
	if strings.TrimSpace(config.Host) == "" || config.Port <= 0 || strings.TrimSpace(config.FromEmail) == "" {
		return nil, errors.New("email config incomplete")
	}
	recipients := normalizeEmailAddresses([]string{input.RecipientEmail})
	if len(recipients) == 0 {
		return nil, errors.New("recipientEmail required")
	}
	templateCode := strings.TrimSpace(input.TemplateCode)
	templateName := "邮件配置调试"
	subject := strings.TrimSpace(input.Subject)
	content := strings.TrimSpace(input.Content)
	variables := defaultEmailTemplateVariables(templateCode)
	variables["email"] = recipients[0]
	for key, value := range input.Values {
		variables[key] = value
	}
	recordTemplateCode := TemplateCodeEmailConfigTest
	if templateCode != "" {
		tpl, err := MessageTemplateServiceApp.GetEnabledEmail(templateCode)
		if err != nil {
			return nil, err
		}
		recordTemplateCode = tpl.Code
		templateName = tpl.Name
		if subject == "" {
			subject = tpl.Subject
		}
		if content == "" {
			content = tpl.Content
		}
	}
	if subject == "" {
		subject = "Nav API 邮件配置测试"
	}
	subject = strings.TrimSpace(utils.RenderTemplateText(subject, variables))
	if subject == "" {
		subject = templateName
	}
	if content == "" {
		content = "这是一封 Nav API 邮件配置调试邮件。如果你收到这封邮件，说明当前 SMTP 配置可以正常发送邮件。"
	}
	content = utils.RenderTemplateText(content, variables)
	htmlBody := utils.DefaultEmailHTML(utils.EmailHTMLInput{
		Title:   templateName,
		Subject: subject,
		Content: content,
	})
	return s.sendWithConfig(config, emailRenderedInput{
		TemplateCode: recordTemplateCode,
		TemplateName: templateName,
		Subject:      subject,
		HTML:         htmlBody,
		Recipients:   recipients,
	})
}

type emailRenderedInput struct {
	TemplateCode string
	TemplateName string
	Subject      string
	HTML         string
	Recipients   []string
}

func (s EmailService) renderTemplate(tpl *domains.MessageTemplate, title string, values map[string]string) (string, string) {
	variables := utils.NormalizeTemplateVariables(values)
	subject := strings.TrimSpace(utils.RenderTemplateText(tpl.Subject, variables))
	if subject == "" {
		subject = utils.FirstNonEmpty(title, tpl.Name, "Nav API 通知")
	}
	body := utils.RenderTemplateText(tpl.Content, variables)
	htmlBody := utils.DefaultEmailHTML(utils.EmailHTMLInput{
		Title:   utils.FirstNonEmpty(title, subject, tpl.Name),
		Subject: subject,
		Content: body,
	})
	return subject, htmlBody
}

func (s EmailService) sendRenderedHTML(input emailRenderedInput) (*EmailSendResult, error) {
	recipients := normalizeEmailAddresses(input.Recipients)
	if len(recipients) == 0 {
		return nil, errors.New("email recipients required")
	}
	subject := strings.TrimSpace(input.Subject)
	if subject == "" {
		return nil, errors.New("email subject required")
	}
	htmlBody := strings.TrimSpace(input.HTML)
	if htmlBody == "" {
		return nil, errors.New("email html required")
	}
	config, configErr := s.defaultEmailConfig()
	batchGuid := uuid.NewString()
	result := &EmailSendResult{Recipients: len(recipients), Subject: subject, BatchGuid: batchGuid}
	for _, recipient := range recipients {
		record := domains.MessageSendRecord{
			BatchGuid:      batchGuid,
			Channel:        MessageChannelEmail,
			TemplateCode:   strings.TrimSpace(input.TemplateCode),
			TemplateName:   strings.TrimSpace(input.TemplateName),
			Subject:        subject,
			RecipientEmail: recipient,
			HTMLContent:    htmlBody,
			SendStatus:     MessageSendStatusPending,
			ReceiveStatus:  MessageReceiveStatusWaiting,
			MaxRetries:     MaxMessageSendRetries,
			LastSendTime:   nowMilli(),
		}
		if config != nil {
			record.FromEmail = config.FromEmail
			record.FromName = config.FromName
		}
		created, err := MessageSendRecordServiceApp.Create(record)
		if err != nil {
			result.Failures++
			appendEmailSendFailure(result, "", recipient, err)
			continue
		}
		result.RecordGuids = append(result.RecordGuids, created.Guid)
		if configErr != nil {
			result.Failures++
			appendEmailSendFailure(result, created.Guid, recipient, configErr)
			_ = markEmailRecordFailed(created.Guid, configErr)
			continue
		}
		if err := emailSendHTML(s, *config, []string{recipient}, subject, htmlBody); err != nil {
			result.Failures++
			appendEmailSendFailure(result, created.Guid, recipient, err)
			_ = markEmailRecordFailed(created.Guid, err)
			continue
		}
		result.Successes++
		_ = markEmailRecordSuccess(created.Guid)
	}
	if result.Failures > 0 {
		return result, fmt.Errorf("email send failed: %d/%d", result.Failures, result.Recipients)
	}
	return result, nil
}

func (s EmailService) sendWithConfig(config domains.MessageEmailConfig, input emailRenderedInput) (*EmailSendResult, error) {
	recipients := normalizeEmailAddresses(input.Recipients)
	if len(recipients) == 0 {
		return nil, errors.New("email recipients required")
	}
	subject := strings.TrimSpace(input.Subject)
	if subject == "" {
		return nil, errors.New("email subject required")
	}
	htmlBody := strings.TrimSpace(input.HTML)
	if htmlBody == "" {
		return nil, errors.New("email html required")
	}
	batchGuid := uuid.NewString()
	result := &EmailSendResult{Recipients: len(recipients), Subject: subject, BatchGuid: batchGuid}
	for _, recipient := range recipients {
		record := domains.MessageSendRecord{
			BatchGuid:      batchGuid,
			Channel:        MessageChannelEmail,
			TemplateCode:   strings.TrimSpace(input.TemplateCode),
			TemplateName:   strings.TrimSpace(input.TemplateName),
			Subject:        subject,
			RecipientEmail: recipient,
			FromEmail:      config.FromEmail,
			FromName:       config.FromName,
			HTMLContent:    htmlBody,
			SendStatus:     MessageSendStatusPending,
			ReceiveStatus:  MessageReceiveStatusWaiting,
			MaxRetries:     MaxMessageSendRetries,
			LastSendTime:   nowMilli(),
		}
		created, err := MessageSendRecordServiceApp.Create(record)
		if err != nil {
			result.Failures++
			appendEmailSendFailure(result, "", recipient, err)
			continue
		}
		result.RecordGuids = append(result.RecordGuids, created.Guid)
		if err := emailSendHTML(s, config, []string{recipient}, subject, htmlBody); err != nil {
			result.Failures++
			appendEmailSendFailure(result, created.Guid, recipient, err)
			_ = markEmailRecordFailed(created.Guid, err)
			continue
		}
		result.Successes++
		_ = markEmailRecordSuccess(created.Guid)
	}
	if result.Failures > 0 {
		return result, emailSendFailedError(result)
	}
	return result, nil
}

func appendEmailSendFailure(result *EmailSendResult, recordGuid string, recipient string, err error) {
	if result == nil || err == nil {
		return
	}
	result.FailureDetails = append(result.FailureDetails, EmailSendFailure{
		RecordGuid:     strings.TrimSpace(recordGuid),
		RecipientEmail: strings.TrimSpace(recipient),
		Error:          err.Error(),
	})
}

func emailSendFailedError(result *EmailSendResult) error {
	if result == nil {
		return errors.New("email send failed")
	}
	message := fmt.Sprintf("email send failed: %d/%d", result.Failures, result.Recipients)
	if len(result.FailureDetails) > 0 && strings.TrimSpace(result.FailureDetails[0].Error) != "" {
		message += ": " + strings.TrimSpace(result.FailureDetails[0].Error)
	}
	return errors.New(message)
}

func markEmailRecordSuccess(guid string) error {
	now := nowMilli()
	return MessageSendRecordServiceApp.UpdateStatus(guid, map[string]any{
		"send_status":     MessageSendStatusSuccess,
		"receive_status":  MessageReceiveStatusDone,
		"error_message":   "",
		"next_retry_time": 0,
		"success_time":    now,
		"last_send_time":  now,
	})
}

func markEmailRecordFailed(guid string, err error) error {
	message := ""
	if err != nil {
		message = err.Error()
	}
	return MessageSendRecordServiceApp.UpdateStatus(guid, map[string]any{
		"send_status":    MessageSendStatusFailed,
		"receive_status": MessageReceiveStatusFailed,
		"error_message":  message,
		"last_send_time": nowMilli(),
	})
}

func (s EmailService) defaultEmailConfig() (*domains.MessageEmailConfig, error) {
	var row domains.MessageEmailConfig
	err := MessageEmailConfigServiceApp.DB().
		Where("status = ?", 1).
		Order("is_default DESC, id DESC").
		First(&row).Error
	if err != nil {
		return nil, errors.New("email config not configured")
	}
	if strings.TrimSpace(row.Host) == "" || row.Port <= 0 || strings.TrimSpace(row.FromEmail) == "" {
		return nil, errors.New("email config incomplete")
	}
	return &row, nil
}

func (s EmailService) sendHTML(config domains.MessageEmailConfig, recipients []string, subject string, htmlBody string) error {
	host := strings.TrimSpace(config.Host)
	addr := net.JoinHostPort(host, fmt.Sprint(config.Port))
	headers := map[string]string{
		"From":         formatEmailAddress(config.FromName, config.FromEmail),
		"To":           strings.Join(recipients, ", "),
		"Subject":      mime.QEncoding.Encode("UTF-8", subject),
		"MIME-Version": "1.0",
		"Content-Type": `text/html; charset="UTF-8"`,
	}
	var message strings.Builder
	for _, key := range []string{"From", "To", "Subject", "MIME-Version", "Content-Type"} {
		message.WriteString(key)
		message.WriteString(": ")
		message.WriteString(headers[key])
		message.WriteString("\r\n")
	}
	message.WriteString("\r\n")
	message.WriteString(htmlBody)
	auth := smtp.Auth(nil)
	username := strings.TrimSpace(config.Username)
	password := strings.TrimSpace(config.Password)
	if username != "" || password != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}
	switch strings.ToLower(strings.TrimSpace(config.Encryption)) {
	case "ssl", "tls":
		return sendMailTLS(addr, host, auth, config.FromEmail, recipients, []byte(message.String()))
	case "starttls":
		return sendMailStartTLS(addr, host, auth, config.FromEmail, recipients, []byte(message.String()))
	default:
		return sendMailPlain(addr, host, auth, config.FromEmail, recipients, []byte(message.String()))
	}
}

func sendMailTLS(addr string, host string, auth smtp.Auth, from string, to []string, msg []byte) error {
	dialer := &net.Dialer{Timeout: emailDialTimeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	if err != nil {
		return err
	}
	defer conn.Close()
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer client.Close()
	return sendSMTP(client, auth, from, to, msg)
}

func sendMailStartTLS(addr string, host string, auth smtp.Auth, from string, to []string, msg []byte) error {
	client, err := dialSMTP(addr, host)
	if err != nil {
		return err
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	return sendSMTP(client, auth, from, to, msg)
}

func sendMailPlain(addr string, host string, auth smtp.Auth, from string, to []string, msg []byte) error {
	client, err := dialSMTP(addr, host)
	if err != nil {
		return err
	}
	defer client.Close()
	return sendSMTP(client, auth, from, to, msg)
}

func dialSMTP(addr string, host string) (*smtp.Client, error) {
	dialer := &net.Dialer{Timeout: emailDialTimeout}
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return client, nil
}

func sendSMTP(client *smtp.Client, auth smtp.Auth, from string, to []string, msg []byte) error {
	if auth != nil {
		if ok, _ := client.Extension("AUTH"); ok {
			if err := client.Auth(auth); err != nil {
				return err
			}
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, addr := range to {
		if err := client.Rcpt(addr); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(msg); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func normalizeEmailAddresses(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		email := strings.TrimSpace(value)
		if email == "" {
			continue
		}
		key := strings.ToLower(email)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, email)
	}
	return result
}

func formatEmailAddress(name string, email string) string {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)
	if name == "" {
		return email
	}
	return mime.QEncoding.Encode("UTF-8", name) + " <" + email + ">"
}

func defaultEmailTemplateVariables(code string) map[string]string {
	now := time.Now()
	values := map[string]string{
		"appName":              "Nav API",
		"username":             "示例用户",
		"email":                "user@example.com",
		"userGuid":             "user-guid-demo",
		"code":                 "123456",
		"ttlMinutes":           strconv.Itoa(int(RegisterEmailCodeTTL.Minutes())),
		"balance":              "120",
		"remainAmount":         "120",
		"usedAmount":           "880",
		"totalAmount":          "1000",
		"threshold":            "200",
		"amountUnit":           "元",
		"reason":               "账户可用余额低于预警阈值",
		"rechargeUrl":          "https://navapi.local/console/wallet",
		"consoleUrl":           "https://navapi.local/console",
		"billDate":             now.Format("2006-01-02"),
		"startTime":            now.Format("2006-01-02") + " 00:00:00",
		"endTime":              now.Format("2006-01-02") + " 23:59:59",
		"requestCount":         "128",
		"successCount":         "126",
		"failedCount":          "2",
		"successRate":          "98.44%",
		"promptTokens":         "1,280,000",
		"completionTokens":     "320,000",
		"totalTokens":          "1,600,000",
		"usageAmount":          "860",
		"topModel":             "gpt-4o-mini",
		"usageDetails":         `<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="width:100%;border-collapse:collapse;"><tr><td style="padding:8px;border-bottom:1px solid #e2e8f0;">gpt-4o-mini</td><td style="padding:8px;border-bottom:1px solid #e2e8f0;text-align:right;">86 次</td><td style="padding:8px;border-bottom:1px solid #e2e8f0;text-align:right;">520 元</td></tr><tr><td style="padding:8px;">text-embedding-3-small</td><td style="padding:8px;text-align:right;">42 次</td><td style="padding:8px;text-align:right;">340 元</td></tr></table>`,
		"adminName":            "管理员",
		"adminEmail":           "admin@example.com",
		"activeUserCount":      "36",
		"newUserCount":         "5",
		"apiKeyCount":          "58",
		"providerCount":        "4",
		"platformAmount":       "12,860",
		"topUser":              "user@example.com",
		"topProvider":          "openai-main",
		"adminUserDetails":     `<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="width:100%;border-collapse:collapse;"><tr><td style="padding:8px;border-bottom:1px solid #e2e8f0;">user@example.com</td><td style="padding:8px;border-bottom:1px solid #e2e8f0;text-align:right;">62 次</td><td style="padding:8px;border-bottom:1px solid #e2e8f0;text-align:right;">4,280 Token</td></tr><tr><td style="padding:8px;">team@example.com</td><td style="padding:8px;text-align:right;">41 次</td><td style="padding:8px;text-align:right;">3,120 Token</td></tr></table>`,
		"adminModelDetails":    `<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="width:100%;border-collapse:collapse;"><tr><td style="padding:8px;border-bottom:1px solid #e2e8f0;">gpt-4o-mini</td><td style="padding:8px;border-bottom:1px solid #e2e8f0;text-align:right;">8,900 Token</td></tr><tr><td style="padding:8px;">gpt-4.1</td><td style="padding:8px;text-align:right;">3,960 Token</td></tr></table>`,
		"adminProviderDetails": `<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="width:100%;border-collapse:collapse;"><tr><td style="padding:8px;border-bottom:1px solid #e2e8f0;">openai-main</td><td style="padding:8px;border-bottom:1px solid #e2e8f0;text-align:right;">9,420 Token</td></tr><tr><td style="padding:8px;">backup-provider</td><td style="padding:8px;text-align:right;">3,440 Token</td></tr></table>`,
		"adminUsageUrl":        "https://navapi.local/admin/operation/usage",
		"usageLogsUrl":         "https://navapi.local/console/usage-logs",
		"time":                 now.Format("2006-01-02 15:04:05"),
	}
	values["templateCode"] = strings.TrimSpace(code)
	return values
}

func randomNumericCode(length int) (string, error) {
	if length <= 0 {
		length = 6
	}
	var builder strings.Builder
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		builder.WriteString(n.String())
	}
	return builder.String(), nil
}
