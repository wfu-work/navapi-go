package services

import (
	"errors"
	"strings"
	"time"

	"navapi-go/constants"
	"navapi-go/domains"
	"navapi-go/dto"

	commonServices "github.com/wfu-work/nav-common-go-lib/services"
	"gorm.io/gorm"
)

type AnnouncementService struct {
	commonServices.CrudService[domains.Announcement]
}

var AnnouncementServiceApp = new(AnnouncementService)

func (s *AnnouncementService) WithDB(db *gorm.DB) *AnnouncementService {
	cloned := *s
	cloned.CrudService = *s.CrudService.WithDB(db)
	return &cloned
}

type AnnouncementQuery struct {
	dto.PageQuery
	Status int    `form:"status" json:"status"`
	Level  string `form:"level" json:"level"`
	Popup  *bool  `form:"popup" json:"popup"`
}

func (s AnnouncementService) List(query AnnouncementQuery, activeOnly bool) (dto.PageResult, error) {
	query.PageQuery.Normalize()
	var announcements []domains.Announcement
	var total int64
	db := s.DB()
	if db == nil {
		return dto.PageResult{}, errors.New("database is not initialized")
	}
	db = db.Model(&domains.Announcement{})
	if activeOnly {
		now := time.Now().Unix()
		db = db.Where("status = ? AND (start_time = 0 OR start_time <= ?) AND (end_time = 0 OR end_time >= ?)", constants.StatusEnabled, now, now)
	}
	if query.Status > 0 {
		db = db.Where("status = ?", query.Status)
	}
	if query.Level != "" {
		db = db.Where("level = ?", strings.TrimSpace(query.Level))
	}
	if query.Popup != nil {
		db = db.Where("popup = ?", *query.Popup)
	}
	if query.Q != "" {
		db = db.Where("title LIKE ? OR content LIKE ? OR level LIKE ? OR remark LIKE ?", "%"+query.Q+"%", "%"+query.Q+"%", "%"+query.Q+"%", "%"+query.Q+"%")
	}
	if err := db.Count(&total).Error; err != nil {
		return dto.PageResult{}, err
	}
	if err := db.Order("priority desc, id desc").Offset(query.PageQuery.Offset()).Limit(query.Size).Find(&announcements).Error; err != nil {
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
	db := s.DB()
	if db == nil {
		return nil, errors.New("database is not initialized")
	}
	err := db.Where("status = ? AND (start_time = 0 OR start_time <= ?) AND (end_time = 0 OR end_time >= ?)", constants.StatusEnabled, now, now).
		Order("priority desc, id desc").
		Limit(limit).
		Find(&announcements).Error
	return announcements, err
}

func (s AnnouncementService) GetByID(id uint) (*domains.Announcement, error) {
	if id == 0 {
		return nil, errors.New("id is required")
	}
	announcement, err := s.GetById(id)
	if err != nil {
		return nil, err
	}
	if announcement == nil {
		return nil, errors.New("announcement not found")
	}
	return announcement, nil
}

// Save normalizes defaults so callers can create simple notices with only a
// title/content while still supporting timed popup announcements.
func (s AnnouncementService) Save(announcement *domains.Announcement) error {
	if err := s.normalize(announcement); err != nil {
		return err
	}
	if announcement.Id == 0 {
		return createWithCrud(&s.CrudService, announcement)
	}
	existing, err := s.GetById(announcement.Id)
	if err != nil {
		return err
	}
	if existing == nil {
		return errors.New("announcement not found")
	}
	announcement.Guid = existing.Guid
	announcement.CreateTime = existing.CreateTime
	announcement.Creater = existing.Creater
	updating := *announcement
	updating.Id = 0
	if err := createWithCrud(&s.CrudService, &updating); err != nil {
		return err
	}
	*announcement = updating
	return nil
}

func (s AnnouncementService) Delete(id uint) error {
	return deleteByIDWithCrud(&s.CrudService, id, "announcement not found")
}

func (s AnnouncementService) normalize(announcement *domains.Announcement) error {
	announcement.Title = strings.TrimSpace(announcement.Title)
	announcement.Content = strings.TrimSpace(announcement.Content)
	announcement.Level = strings.TrimSpace(announcement.Level)
	announcement.Remark = strings.TrimSpace(announcement.Remark)

	if announcement.Title == "" {
		return errors.New("title is required")
	}
	if announcement.Content == "" {
		return errors.New("content is required")
	}
	if announcement.Level == "" {
		announcement.Level = "info"
	}
	if !isAnnouncementLevel(announcement.Level) {
		return errors.New("level must be info, warning or error")
	}
	if announcement.Status == 0 {
		announcement.Status = constants.StatusEnabled
	}
	if announcement.Status != constants.StatusEnabled && announcement.Status != constants.StatusDisabled {
		return errors.New("status is invalid")
	}
	if announcement.StartTime < 0 || announcement.EndTime < 0 {
		return errors.New("start time and end time cannot be negative")
	}
	if announcement.StartTime > 0 && announcement.EndTime > 0 && announcement.EndTime < announcement.StartTime {
		return errors.New("end time cannot be earlier than start time")
	}
	return nil
}

func isAnnouncementLevel(level string) bool {
	return level == "info" || level == "warning" || level == "error"
}
