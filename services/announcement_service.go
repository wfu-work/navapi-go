package services

import (
	"errors"
	"strings"
	"time"

	"navapi-go/constants"
	"navapi-go/domains"
	"navapi-go/dto"

	"github.com/wfu-work/nav-common-go-lib/global"
)

type AnnouncementService struct{}

var AnnouncementServiceApp = AnnouncementService{}

func (s AnnouncementService) List(query dto.PageQuery, activeOnly bool) (dto.PageResult, error) {
	query.Normalize()
	var announcements []domains.Announcement
	var total int64
	db := global.NAV_DB.Model(&domains.Announcement{})
	if activeOnly {
		now := time.Now().Unix()
		db = db.Where("status = ? AND (start_time = 0 OR start_time <= ?) AND (end_time = 0 OR end_time >= ?)", constants.StatusEnabled, now, now)
	}
	if query.Q != "" {
		db = db.Where("title LIKE ? OR content LIKE ? OR level LIKE ? OR remark LIKE ?", "%"+query.Q+"%", "%"+query.Q+"%", "%"+query.Q+"%", "%"+query.Q+"%")
	}
	if err := db.Count(&total).Error; err != nil {
		return dto.PageResult{}, err
	}
	if err := db.Order("priority desc, id desc").Offset(query.Offset()).Limit(query.Size).Find(&announcements).Error; err != nil {
		return dto.PageResult{}, err
	}
	return dto.PageResult{List: announcements, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s AnnouncementService) Latest(limit int) ([]domains.Announcement, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}
	now := time.Now().Unix()
	var announcements []domains.Announcement
	err := global.NAV_DB.Where("status = ? AND (start_time = 0 OR start_time <= ?) AND (end_time = 0 OR end_time >= ?)", constants.StatusEnabled, now, now).
		Order("priority desc, id desc").
		Limit(limit).
		Find(&announcements).Error
	return announcements, err
}

func (s AnnouncementService) GetByID(id uint) (*domains.Announcement, error) {
	var announcement domains.Announcement
	if err := global.NAV_DB.First(&announcement, id).Error; err != nil {
		return nil, err
	}
	return &announcement, nil
}

// Save normalizes defaults so callers can create simple notices with only a
// title/content while still supporting timed popup announcements.
func (s AnnouncementService) Save(announcement *domains.Announcement) error {
	if strings.TrimSpace(announcement.Title) == "" {
		return errors.New("title is required")
	}
	if strings.TrimSpace(announcement.Level) == "" {
		announcement.Level = "info"
	}
	if announcement.Status == 0 {
		announcement.Status = constants.StatusEnabled
	}
	return global.NAV_DB.Save(announcement).Error
}

func (s AnnouncementService) Delete(id uint) error {
	return global.NAV_DB.Delete(&domains.Announcement{}, id).Error
}
