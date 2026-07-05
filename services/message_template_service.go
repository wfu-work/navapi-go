package services

import (
	"errors"
	"strconv"
	"strings"

	"navapi-go/constants"
	"navapi-go/domains"
	"navapi-go/static"
	"navapi-go/vos"

	commonDomains "github.com/wfu-work/nav-common-go-lib/domains"
	commonServices "github.com/wfu-work/nav-common-go-lib/services"
	"gorm.io/gorm"
)

type MessageTemplateService struct {
	commonServices.CrudService[domains.MessageTemplate]
}

var MessageTemplateServiceApp = new(MessageTemplateService)

type SaveMessageTemplateRequest struct {
	Guid        string `json:"guid"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Channel     string `json:"channel"`
	Subject     string `json:"subject"`
	Content     string `json:"content"`
	Description string `json:"description"`
	Status      int    `json:"status"`
}

func (s *MessageTemplateService) WithDB(db *gorm.DB) *MessageTemplateService {
	cloned := *s
	cloned.CrudService = *s.CrudService.WithDB(db)
	return &cloned
}

func (s *MessageTemplateService) List(query vos.PageQuery, channel string, status string) (vos.PageResult, error) {
	query.Normalize()
	var rows []domains.MessageTemplate
	var total int64
	db := s.DB().Model(&domains.MessageTemplate{})
	if query.Q != "" {
		keyword := "%" + query.Q + "%"
		db = db.Where("code LIKE ? OR name LIKE ? OR subject LIKE ? OR description LIKE ?", keyword, keyword, keyword, keyword)
	}
	if strings.TrimSpace(channel) != "" {
		db = db.Where("channel = ?", strings.TrimSpace(channel))
	}
	if strings.TrimSpace(status) != "" {
		value, err := strconv.Atoi(strings.TrimSpace(status))
		if err != nil {
			return vos.PageResult{}, err
		}
		db = db.Where("status = ?", value)
	}
	if err := db.Count(&total).Error; err != nil {
		return vos.PageResult{}, err
	}
	if err := db.Order("id desc").Offset(query.Offset()).Limit(query.Size).Find(&rows).Error; err != nil {
		return vos.PageResult{}, err
	}
	return vos.PageResult{List: rows, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s *MessageTemplateService) Get(identity string) (*domains.MessageTemplate, error) {
	identity = strings.TrimSpace(identity)
	if identity == "" {
		return nil, errors.New("template identity required")
	}
	var row domains.MessageTemplate
	if err := s.DB().Where("guid = ? OR code = ?", identity, identity).First(&row).Error; err != nil {
		return nil, errors.New("template not found")
	}
	return &row, nil
}

func (s *MessageTemplateService) GetEnabledEmail(code string) (*domains.MessageTemplate, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, errors.New("template code required")
	}
	var row domains.MessageTemplate
	if err := s.DB().Where("code = ? AND channel = ? AND status = ?", code, MessageChannelEmail, constants.StatusEnabled).First(&row).Error; err != nil {
		return nil, errors.New("enabled email template not found")
	}
	return &row, nil
}

func (s *MessageTemplateService) Save(req SaveMessageTemplateRequest) (*domains.MessageTemplate, error) {
	req = normalizeTemplateRequest(req)
	if req.Code == "" {
		return nil, errors.New("code required")
	}
	if req.Subject == "" {
		return nil, errors.New("subject required")
	}
	if req.Content == "" {
		return nil, errors.New("content required")
	}
	if err := s.ensureCodeAvailable(req.Code, req.Guid); err != nil {
		return nil, err
	}
	now := nowMilli()
	var row domains.MessageTemplate
	query := s.DB()
	if req.Guid != "" {
		query = query.Where("guid = ?", req.Guid)
	} else {
		query = query.Where("code = ?", req.Code)
	}
	err := query.First(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = domains.MessageTemplate{BaseDataEntity: commonDomains.BaseDataEntity{CreateTime: now}}
		if req.Guid != "" {
			row.Guid = req.Guid
		}
	}
	row.Code = req.Code
	row.Name = req.Name
	row.Channel = req.Channel
	row.Subject = req.Subject
	row.Content = req.Content
	row.Description = req.Description
	row.Status = req.Status
	row.UpdateTime = now
	if err := s.DB().Save(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *MessageTemplateService) Disable(guid string) error {
	return disableMessageEntity(s.DB(), &domains.MessageTemplate{}, guid)
}

func (s *MessageTemplateService) SeedDefaults() {
	if s.DB() == nil {
		return
	}
	now := nowMilli()
	for _, item := range defaultMessageTemplates(now) {
		var count int64
		if err := s.DB().Model(&domains.MessageTemplate{}).Where("code = ?", item.Code).Count(&count).Error; err != nil || count > 0 {
			continue
		}
		_ = s.DB().Create(&item).Error
	}
}

func (s *MessageTemplateService) ensureCodeAvailable(code string, currentGuid string) error {
	var existing domains.MessageTemplate
	err := s.DB().Where("code = ?", code).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if currentGuid != "" && existing.Guid == currentGuid {
		return nil
	}
	return errors.New("code already exists")
}

func defaultMessageTemplates(now int64) []domains.MessageTemplate {
	return []domains.MessageTemplate{
		{
			BaseDataEntity: commonDomains.BaseDataEntity{Guid: TemplateCodeRegisterCaptcha, CreateTime: now, UpdateTime: now},
			Code:           TemplateCodeRegisterCaptcha,
			Name:           "用户注册验证码",
			Channel:        MessageChannelEmail,
			Subject:        "{{appName}} 注册验证码：{{code}}",
			Content:        strings.TrimSpace(static.RegisterEmailCodeTemplateHTML),
			Description:    "客户端用户注册时发送邮箱验证码，模板编码不要修改。",
			Status:         constants.StatusEnabled,
		},
		{
			BaseDataEntity: commonDomains.BaseDataEntity{Guid: TemplateCodeUserBalanceInsufficient, CreateTime: now, UpdateTime: now},
			Code:           TemplateCodeUserBalanceInsufficient,
			Name:           "用户余额不足提醒",
			Channel:        MessageChannelEmail,
			Subject:        "{{appName}} 用户余额不足提醒",
			Content:        strings.TrimSpace(static.UserBalanceInsufficientTemplateHTML),
			Description:    "用户账户余额或额度低于阈值时发送提醒，模板编码不要修改。",
			Status:         constants.StatusEnabled,
		},
		{
			BaseDataEntity: commonDomains.BaseDataEntity{Guid: TemplateCodeUserDailyUsageBill, CreateTime: now, UpdateTime: now},
			Code:           TemplateCodeUserDailyUsageBill,
			Name:           "普通用户每日用量账单",
			Channel:        MessageChannelEmail,
			Subject:        "{{appName}} 每日用量账单：{{billDate}}",
			Content:        strings.TrimSpace(static.UserDailyUsageBillTemplateHTML),
			Description:    "普通用户每日 API 调用、Token 与额度消耗账单邮件，模板编码不要修改。",
			Status:         constants.StatusEnabled,
		},
		{
			BaseDataEntity: commonDomains.BaseDataEntity{Guid: TemplateCodeAdminDailyUsageBill, CreateTime: now, UpdateTime: now},
			Code:           TemplateCodeAdminDailyUsageBill,
			Name:           "管理员每日用量账单",
			Channel:        MessageChannelEmail,
			Subject:        "{{appName}} 管理员每日用量账单：{{billDate}}",
			Content:        strings.TrimSpace(static.AdminDailyUsageBillTemplateHTML),
			Description:    "管理员每日平台调用、用户、模型与渠道用量汇总账单邮件，模板编码不要修改。",
			Status:         constants.StatusEnabled,
		},
	}
}

func normalizeTemplateRequest(req SaveMessageTemplateRequest) SaveMessageTemplateRequest {
	req.Guid = strings.TrimSpace(req.Guid)
	req.Code = strings.TrimSpace(req.Code)
	req.Name = strings.TrimSpace(req.Name)
	req.Channel = strings.TrimSpace(req.Channel)
	req.Subject = strings.TrimSpace(req.Subject)
	req.Content = strings.TrimSpace(req.Content)
	req.Description = strings.TrimSpace(req.Description)
	if req.Channel == "" {
		req.Channel = MessageChannelEmail
	}
	if req.Name == "" {
		req.Name = req.Code
	}
	if req.Status == 0 {
		req.Status = constants.StatusEnabled
	}
	return req
}
