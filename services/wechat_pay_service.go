package services

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"navapi-go/domains"

	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/downloader"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
	sdkUtils "github.com/wechatpay-apiv3/wechatpay-go/utils"
)

const (
	paymentProviderWechat = "wechat"
	wechatTradeTypeNative = "NATIVE"

	optionWechatPayEnabled                = "payment.wechat.enabled"
	optionWechatPayAppID                  = "payment.wechat.app_id"
	optionWechatPayMchID                  = "payment.wechat.mch_id"
	optionWechatPayMchCertificateSerialNo = "payment.wechat.mch_certificate_serial_no"
	optionWechatPayMchPrivateKey          = "payment.wechat.mch_private_key"
	optionWechatPayMchPrivateKeyPath      = "payment.wechat.mch_private_key_path"
	optionWechatPayAPIv3Key               = "payment.wechat.api_v3_key"
	optionWechatPayNotifyURL              = "payment.wechat.notify_url"
	optionWechatPayDescriptionPrefix      = "payment.wechat.description_prefix"
)

type WechatPaySettings struct {
	Enabled                bool   `json:"enabled"`
	AppID                  string `json:"appId"`
	MchID                  string `json:"mchId"`
	MchCertificateSerialNo string `json:"mchCertificateSerialNo"`
	MchPrivateKey          string `json:"mchPrivateKey"`
	MchPrivateKeyPath      string `json:"mchPrivateKeyPath"`
	APIv3Key               string `json:"apiV3Key"`
	NotifyURL              string `json:"notifyUrl"`
	DescriptionPrefix      string `json:"descriptionPrefix"`
}

func (s *PaymentService) GetWechatPaySettings() WechatPaySettings {
	settings := WechatPaySettings{
		Enabled:                OptionServiceApp.Int64(optionWechatPayEnabled, 0) > 0,
		AppID:                  OptionServiceApp.Get(optionWechatPayAppID, ""),
		MchID:                  OptionServiceApp.Get(optionWechatPayMchID, ""),
		MchCertificateSerialNo: OptionServiceApp.Get(optionWechatPayMchCertificateSerialNo, ""),
		MchPrivateKey:          OptionServiceApp.Get(optionWechatPayMchPrivateKey, ""),
		MchPrivateKeyPath:      OptionServiceApp.Get(optionWechatPayMchPrivateKeyPath, ""),
		APIv3Key:               OptionServiceApp.Get(optionWechatPayAPIv3Key, ""),
		NotifyURL:              OptionServiceApp.Get(optionWechatPayNotifyURL, ""),
		DescriptionPrefix:      OptionServiceApp.Get(optionWechatPayDescriptionPrefix, "NavAPI"),
	}
	settings.normalize()
	return settings
}

func (s *PaymentService) SetWechatPaySettings(settings WechatPaySettings) error {
	settings.normalize()
	current := s.GetWechatPaySettings()
	if isMaskedSecret(settings.MchPrivateKey) || settings.MchPrivateKey == "" {
		settings.MchPrivateKey = current.MchPrivateKey
	}
	if isMaskedSecret(settings.APIv3Key) || settings.APIv3Key == "" {
		settings.APIv3Key = current.APIv3Key
	}
	if settings.Enabled {
		if err := settings.validateForPrepay(); err != nil {
			return err
		}
	}
	values := map[string]string{
		optionWechatPayEnabled:                boolOption(settings.Enabled),
		optionWechatPayAppID:                  settings.AppID,
		optionWechatPayMchID:                  settings.MchID,
		optionWechatPayMchCertificateSerialNo: settings.MchCertificateSerialNo,
		optionWechatPayMchPrivateKey:          settings.MchPrivateKey,
		optionWechatPayMchPrivateKeyPath:      settings.MchPrivateKeyPath,
		optionWechatPayAPIv3Key:               settings.APIv3Key,
		optionWechatPayNotifyURL:              settings.NotifyURL,
		optionWechatPayDescriptionPrefix:      settings.DescriptionPrefix,
	}
	for key, value := range values {
		if err := OptionServiceApp.Set(key, value); err != nil {
			return err
		}
	}
	return nil
}

func (settings WechatPaySettings) MaskSecrets() WechatPaySettings {
	if settings.MchPrivateKey != "" {
		settings.MchPrivateKey = "******"
	}
	if settings.APIv3Key != "" {
		settings.APIv3Key = "******"
	}
	return settings
}

func (settings *WechatPaySettings) normalize() {
	settings.AppID = strings.TrimSpace(settings.AppID)
	settings.MchID = strings.TrimSpace(settings.MchID)
	settings.MchCertificateSerialNo = strings.TrimSpace(settings.MchCertificateSerialNo)
	settings.MchPrivateKey = normalizePrivateKey(settings.MchPrivateKey)
	settings.MchPrivateKeyPath = strings.TrimSpace(settings.MchPrivateKeyPath)
	settings.APIv3Key = strings.TrimSpace(settings.APIv3Key)
	settings.NotifyURL = strings.TrimSpace(settings.NotifyURL)
	settings.DescriptionPrefix = strings.TrimSpace(settings.DescriptionPrefix)
	if settings.DescriptionPrefix == "" {
		settings.DescriptionPrefix = "NavAPI"
	}
}

func (settings WechatPaySettings) validateForPrepay() error {
	if !settings.Enabled {
		return errors.New("wechat pay is disabled")
	}
	if settings.AppID == "" {
		return errors.New("wechat pay appId is required")
	}
	if settings.MchID == "" {
		return errors.New("wechat pay mchId is required")
	}
	if settings.MchCertificateSerialNo == "" {
		return errors.New("wechat pay merchant certificate serial no is required")
	}
	if settings.MchPrivateKey == "" && settings.MchPrivateKeyPath == "" {
		return errors.New("wechat pay merchant private key is required")
	}
	if settings.APIv3Key == "" {
		return errors.New("wechat pay api v3 key is required")
	}
	if settings.NotifyURL == "" {
		return errors.New("wechat pay notifyUrl is required")
	}
	return nil
}

func (settings WechatPaySettings) loadPrivateKey() (*rsa.PrivateKey, error) {
	if settings.MchPrivateKey != "" {
		return sdkUtils.LoadPrivateKey(settings.MchPrivateKey)
	}
	return sdkUtils.LoadPrivateKeyWithPath(settings.MchPrivateKeyPath)
}

func (s *PaymentService) createWechatNativePrepay(ctx context.Context, order *domains.PaymentOrder) error {
	if order == nil {
		return errors.New("payment order is required")
	}
	if order.AmountCents <= 0 {
		return errors.New("amountCents must be greater than zero")
	}
	settings := s.GetWechatPaySettings()
	if err := settings.validateForPrepay(); err != nil {
		return err
	}
	ctx, cancel := wechatPayContext(ctx)
	defer cancel()

	client, err := wechatPayClient(ctx, settings)
	if err != nil {
		return err
	}
	svc := native.NativeApiService{Client: client}
	resp, _, err := svc.Prepay(ctx, native.PrepayRequest{
		Appid:       core.String(settings.AppID),
		Mchid:       core.String(settings.MchID),
		Description: core.String(wechatPayDescription(settings, order)),
		OutTradeNo:  core.String(order.OrderNo),
		Attach:      core.String(order.Guid),
		NotifyUrl:   core.String(settings.NotifyURL),
		Amount: &native.Amount{
			Total:    core.Int64(order.AmountCents),
			Currency: core.String(normalizeCurrency(order.Currency)),
		},
	})
	if err != nil {
		return err
	}
	if resp == nil || resp.CodeUrl == nil || strings.TrimSpace(*resp.CodeUrl) == "" {
		return errors.New("wechat pay prepay response missing code_url")
	}
	order.TradeType = wechatTradeTypeNative
	order.CodeURL = strings.TrimSpace(*resp.CodeUrl)
	order.PrepayID = ""
	if order.Guid == "" {
		return nil
	}
	if err := s.DB().Model(&domains.PaymentOrder{}).Where("guid = ?", order.Guid).Updates(map[string]any{
		"trade_type": order.TradeType,
		"code_url":   order.CodeURL,
		"prepay_id":  order.PrepayID,
	}).Error; err != nil {
		return err
	}
	return reloadByGuidWithCrud(&s.CrudService, order)
}

func (s *PaymentService) validateWechatPaymentReady() error {
	settings := s.GetWechatPaySettings()
	return settings.validateForPrepay()
}

func (s *PaymentService) HandleWechatNotify(request *http.Request) (*domains.PaymentOrder, error) {
	if request == nil {
		return nil, errors.New("request is required")
	}
	ctx, cancel := wechatPayContext(request.Context())
	defer cancel()

	settings := s.GetWechatPaySettings()
	if err := settings.validateForPrepay(); err != nil {
		return nil, err
	}
	handler, err := wechatNotifyHandler(ctx, settings)
	if err != nil {
		return nil, err
	}
	transaction := new(payments.Transaction)
	notifyReq, err := handler.ParseNotifyRequest(ctx, request, transaction)
	if err != nil {
		return nil, err
	}
	notifyData := ""
	if notifyReq != nil && notifyReq.Resource != nil {
		notifyData = notifyReq.Resource.Plaintext
	}
	return s.ConfirmWechatTransaction(transaction, notifyData)
}

func (s *PaymentService) ConfirmWechatTransaction(transaction *payments.Transaction, notifyData string) (*domains.PaymentOrder, error) {
	if transaction == nil {
		return nil, errors.New("wechat transaction is required")
	}
	if stringValue(transaction.TradeState) != "SUCCESS" {
		return nil, fmt.Errorf("wechat trade state is %s", stringValue(transaction.TradeState))
	}
	orderNo := stringValue(transaction.OutTradeNo)
	if orderNo == "" {
		return nil, errors.New("wechat out_trade_no is required")
	}
	transactionID := stringValue(transaction.TransactionId)
	if transactionID == "" {
		return nil, errors.New("wechat transaction_id is required")
	}
	var order domains.PaymentOrder
	if err := s.DB().Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return nil, err
	}
	if !isWechatPaymentProvider(order.Provider) {
		return nil, errors.New("payment order provider is not wechat")
	}
	settings := s.GetWechatPaySettings()
	if settings.MchID != "" && transaction.Mchid != nil && stringValue(transaction.Mchid) != settings.MchID {
		return nil, errors.New("wechat mchid does not match settings")
	}
	if settings.AppID != "" && transaction.Appid != nil && stringValue(transaction.Appid) != settings.AppID {
		return nil, errors.New("wechat appid does not match settings")
	}
	if transaction.Amount != nil && transaction.Amount.Total != nil && *transaction.Amount.Total != order.AmountCents {
		return nil, errors.New("wechat paid amount does not match order amount")
	}
	if transaction.Amount != nil && transaction.Amount.Currency != nil && !strings.EqualFold(stringValue(transaction.Amount.Currency), normalizeCurrency(order.Currency)) {
		return nil, errors.New("wechat paid currency does not match order currency")
	}
	if strings.TrimSpace(notifyData) == "" {
		encoded, _ := json.Marshal(transaction)
		notifyData = string(encoded)
	}
	return s.Confirm(ConfirmPaymentRequest{
		OrderNo:       orderNo,
		TransactionID: transactionID,
		NotifyData:    notifyData,
	})
}

func wechatPayClient(ctx context.Context, settings WechatPaySettings) (*core.Client, error) {
	privateKey, err := settings.loadPrivateKey()
	if err != nil {
		return nil, err
	}
	return core.NewClient(ctx, option.WithWechatPayAutoAuthCipher(
		settings.MchID,
		settings.MchCertificateSerialNo,
		privateKey,
		settings.APIv3Key,
	))
}

func wechatNotifyHandler(ctx context.Context, settings WechatPaySettings) (*notify.Handler, error) {
	privateKey, err := settings.loadPrivateKey()
	if err != nil {
		return nil, err
	}
	mgr := downloader.MgrInstance()
	if !mgr.HasDownloader(ctx, settings.MchID) {
		if err := mgr.RegisterDownloaderWithPrivateKey(ctx, privateKey, settings.MchCertificateSerialNo, settings.MchID, settings.APIv3Key); err != nil {
			return nil, err
		}
	}
	return notify.NewRSANotifyHandler(
		settings.APIv3Key,
		verifiers.NewSHA256WithRSAVerifier(mgr.GetCertificateVisitor(settings.MchID)),
	)
}

func wechatPayContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, 20*time.Second)
}

func isWechatPaymentProvider(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case paymentProviderWechat, "wechat_native", "wxpay", "wx_pay":
		return true
	default:
		return false
	}
}

func wechatPayDescription(settings WechatPaySettings, order *domains.PaymentOrder) string {
	subject := "余额充值"
	if order.Type == "subscription" {
		subject = "订阅购买"
	}
	description := strings.TrimSpace(settings.DescriptionPrefix + " " + subject)
	if len([]rune(description)) <= 127 {
		return description
	}
	runes := []rune(description)
	return string(runes[:127])
}

func normalizeCurrency(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return "CNY"
	}
	return value
}

func normalizePrivateKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !strings.Contains(value, "\n") && strings.Contains(value, `\n`) {
		value = strings.ReplaceAll(value, `\n`, "\n")
	}
	return value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func boolOption(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func isMaskedSecret(value string) bool {
	value = strings.TrimSpace(value)
	return value == "******" || value == "********"
}
