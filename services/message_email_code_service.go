package services

import (
	"errors"
	"strings"

	"navapi-go/domains"

	commonServices "github.com/wfu-work/nav-common-go-lib/services"
	"gorm.io/gorm"
)

type MessageEmailCodeService struct {
	commonServices.CrudService[domains.MessageEmailCode]
}

var MessageEmailCodeServiceApp = new(MessageEmailCodeService)

func (s *MessageEmailCodeService) WithDB(db *gorm.DB) *MessageEmailCodeService {
	cloned := *s
	cloned.CrudService = *s.CrudService.WithDB(db)
	return &cloned
}

func (s *MessageEmailCodeService) Save(code domains.MessageEmailCode) (*domains.MessageEmailCode, error) {
	code.Email = strings.ToLower(strings.TrimSpace(code.Email))
	code.Scene = strings.TrimSpace(code.Scene)
	code.Code = strings.TrimSpace(code.Code)
	if code.Email == "" {
		return nil, errors.New("email required")
	}
	if code.Scene == "" {
		return nil, errors.New("scene required")
	}
	if code.Code == "" {
		return nil, errors.New("code required")
	}
	now := nowMilli()
	if code.Status == "" {
		code.Status = MessageEmailCodePending
	}
	code.CreateTime = now
	code.UpdateTime = now
	if err := s.ExpirePending(code.Email, code.Scene); err != nil {
		return nil, err
	}
	if err := s.DB().Create(&code).Error; err != nil {
		return nil, err
	}
	return &code, nil
}

func (s *MessageEmailCodeService) HasRecentPending(email string, scene string, since int64) (bool, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	scene = strings.TrimSpace(scene)
	if email == "" || scene == "" {
		return false, nil
	}
	var count int64
	err := s.DB().Model(&domains.MessageEmailCode{}).
		Where("email = ? AND scene = ? AND status = ? AND create_time >= ?", email, scene, MessageEmailCodePending, since).
		Count(&count).Error
	return count > 0, err
}

func (s *MessageEmailCodeService) ExpirePending(email string, scene string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	scene = strings.TrimSpace(scene)
	if email == "" || scene == "" {
		return nil
	}
	now := nowMilli()
	return s.DB().Model(&domains.MessageEmailCode{}).
		Where("email = ? AND scene = ? AND status = ?", email, scene, MessageEmailCodePending).
		Updates(map[string]any{"status": MessageEmailCodeExpired, "update_time": now}).Error
}

func (s *MessageEmailCodeService) Verify(email string, scene string, code string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	scene = strings.TrimSpace(scene)
	code = strings.TrimSpace(code)
	if email == "" || scene == "" || code == "" {
		return errors.New("email code required")
	}
	now := nowMilli()
	var row domains.MessageEmailCode
	err := s.DB().
		Where("email = ? AND scene = ? AND code = ? AND status = ? AND expires_time >= ?", email, scene, code, MessageEmailCodePending, now).
		Order("id desc").
		First(&row).Error
	if err != nil {
		return errors.New("email code is invalid or expired")
	}
	return s.DB().Model(&domains.MessageEmailCode{}).Where("guid = ?", row.Guid).Updates(map[string]any{
		"status":      MessageEmailCodeUsed,
		"used_time":   now,
		"update_time": now,
	}).Error
}

func (s *MessageEmailCodeService) ExpireOld() error {
	now := nowMilli()
	return s.DB().Model(&domains.MessageEmailCode{}).
		Where("status = ? AND expires_time < ?", MessageEmailCodePending, now).
		Updates(map[string]any{"status": MessageEmailCodeExpired, "update_time": now}).Error
}
