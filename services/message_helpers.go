package services

import (
	"errors"
	"strings"

	"navapi-go/constants"

	"gorm.io/gorm"
)

func disableMessageEntity(db *gorm.DB, model any, guid string) error {
	guid = strings.TrimSpace(guid)
	if guid == "" {
		return errors.New("guid required")
	}
	return db.Model(model).Where("guid = ?", guid).Updates(map[string]any{
		"status":      constants.StatusDisabled,
		"update_time": nowMilli(),
	}).Error
}
