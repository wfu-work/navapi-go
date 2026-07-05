package services

import (
	"errors"
	"strings"

	"navapi-go/domains"
	"navapi-go/dto"
	"navapi-go/utils"

	commonServices "github.com/wfu-work/nav-common-go-lib/services"
	"gorm.io/gorm"
)

type MessageSendRecordService struct {
	commonServices.CrudService[domains.MessageSendRecord]
}

var MessageSendRecordServiceApp = new(MessageSendRecordService)

func (s *MessageSendRecordService) WithDB(db *gorm.DB) *MessageSendRecordService {
	cloned := *s
	cloned.CrudService = *s.CrudService.WithDB(db)
	return &cloned
}

func (s *MessageSendRecordService) List(query dto.PageQuery, sendStatus string, templateCode string) (dto.PageResult, error) {
	query.Normalize()
	var rows []domains.MessageSendRecord
	var total int64
	db := s.DB().Model(&domains.MessageSendRecord{})
	if query.Q != "" {
		keyword := "%" + query.Q + "%"
		db = db.Where("subject LIKE ? OR template_code LIKE ? OR template_name LIKE ? OR recipient_email LIKE ? OR error_message LIKE ? OR batch_guid LIKE ?", keyword, keyword, keyword, keyword, keyword, keyword)
	}
	if strings.TrimSpace(sendStatus) != "" {
		db = db.Where("send_status = ?", strings.TrimSpace(sendStatus))
	}
	if strings.TrimSpace(templateCode) != "" {
		db = db.Where("template_code = ?", strings.TrimSpace(templateCode))
	}
	if err := db.Count(&total).Error; err != nil {
		return dto.PageResult{}, err
	}
	if err := db.Order("id desc").Offset(query.Offset()).Limit(query.Size).Find(&rows).Error; err != nil {
		return dto.PageResult{}, err
	}
	return dto.PageResult{List: rows, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s *MessageSendRecordService) Get(guid string) (*domains.MessageSendRecord, error) {
	guid = strings.TrimSpace(guid)
	if guid == "" {
		return nil, errors.New("guid required")
	}
	var row domains.MessageSendRecord
	if err := s.DB().Where("guid = ?", guid).First(&row).Error; err != nil {
		return nil, errors.New("send record not found")
	}
	return &row, nil
}

func (s *MessageSendRecordService) Create(record domains.MessageSendRecord) (*domains.MessageSendRecord, error) {
	if strings.TrimSpace(record.TemplateCode) == "" {
		return nil, errors.New("templateCode required")
	}
	if strings.TrimSpace(record.RecipientEmail) == "" {
		return nil, errors.New("recipientEmail required")
	}
	now := nowMilli()
	if record.MaxRetries <= 0 {
		record.MaxRetries = MaxMessageSendRetries
	}
	if record.SendStatus == "" {
		record.SendStatus = MessageSendStatusPending
	}
	if record.ReceiveStatus == "" {
		record.ReceiveStatus = MessageReceiveStatusWaiting
	}
	record.Channel = utils.FirstNonEmpty(record.Channel, MessageChannelEmail)
	record.TemplateCode = strings.TrimSpace(record.TemplateCode)
	record.TemplateName = strings.TrimSpace(record.TemplateName)
	record.Subject = strings.TrimSpace(record.Subject)
	record.RecipientEmail = strings.TrimSpace(record.RecipientEmail)
	record.FromEmail = strings.TrimSpace(record.FromEmail)
	record.FromName = strings.TrimSpace(record.FromName)
	record.HTMLContent = strings.TrimSpace(record.HTMLContent)
	record.ErrorMessage = strings.TrimSpace(record.ErrorMessage)
	record.CreateTime = now
	record.UpdateTime = now
	if err := s.DB().Create(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *MessageSendRecordService) UpdateStatus(guid string, updates map[string]any) error {
	guid = strings.TrimSpace(guid)
	if guid == "" {
		return errors.New("guid required")
	}
	if len(updates) == 0 {
		return nil
	}
	updates["update_time"] = nowMilli()
	return s.DB().Model(&domains.MessageSendRecord{}).Where("guid = ?", guid).Updates(updates).Error
}
