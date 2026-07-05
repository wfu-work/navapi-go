package services

import (
	"errors"
	"strconv"
	"strings"

	"navapi-go/constants"
	"navapi-go/domains"
	"navapi-go/vos"

	commonDomains "github.com/wfu-work/nav-common-go-lib/domains"
	commonServices "github.com/wfu-work/nav-common-go-lib/services"
	"gorm.io/gorm"
)

type MessageEmailConfigService struct {
	commonServices.CrudService[domains.MessageEmailConfig]
}

var MessageEmailConfigServiceApp = new(MessageEmailConfigService)

type SaveMessageEmailConfigRequest struct {
	Guid       string `json:"guid"`
	Name       string `json:"name"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	FromEmail  string `json:"fromEmail"`
	FromName   string `json:"fromName"`
	Encryption string `json:"encryption"`
	IsDefault  bool   `json:"isDefault"`
	Remark     string `json:"remark"`
	Status     int    `json:"status"`
}

func (s *MessageEmailConfigService) WithDB(db *gorm.DB) *MessageEmailConfigService {
	cloned := *s
	cloned.CrudService = *s.CrudService.WithDB(db)
	return &cloned
}

func (s *MessageEmailConfigService) List(query vos.PageQuery, status string) (vos.PageResult, error) {
	query.Normalize()
	var rows []domains.MessageEmailConfig
	var total int64
	db := s.DB().Model(&domains.MessageEmailConfig{})
	if query.Q != "" {
		keyword := "%" + query.Q + "%"
		db = db.Where("name LIKE ? OR host LIKE ? OR username LIKE ? OR from_email LIKE ? OR remark LIKE ?", keyword, keyword, keyword, keyword, keyword)
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
	if err := db.Order("is_default desc, id desc").Offset(query.Offset()).Limit(query.Size).Find(&rows).Error; err != nil {
		return vos.PageResult{}, err
	}
	return vos.PageResult{List: rows, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s *MessageEmailConfigService) Save(req SaveMessageEmailConfigRequest) (*domains.MessageEmailConfig, error) {
	req = normalizeEmailConfigRequest(req)
	if req.Name == "" {
		return nil, errors.New("name required")
	}
	if req.Host == "" {
		return nil, errors.New("host required")
	}
	if req.Port <= 0 {
		return nil, errors.New("port required")
	}
	if req.FromEmail == "" {
		return nil, errors.New("fromEmail required")
	}
	now := nowMilli()
	var row domains.MessageEmailConfig
	db := s.DB()
	err := db.Where("guid = ?", req.Guid).First(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = domains.MessageEmailConfig{BaseDataEntity: commonDomains.BaseDataEntity{CreateTime: now}}
		if req.Guid != "" {
			row.Guid = req.Guid
		}
	}
	row.Name = req.Name
	row.Host = req.Host
	row.Port = req.Port
	row.Username = req.Username
	if req.Password != "" {
		row.Password = req.Password
	}
	row.FromEmail = req.FromEmail
	row.FromName = req.FromName
	row.Encryption = req.Encryption
	row.IsDefault = req.IsDefault
	row.Remark = req.Remark
	row.Status = req.Status
	row.UpdateTime = now
	if err := db.Transaction(func(tx *gorm.DB) error {
		if row.IsDefault {
			if err := tx.Model(&domains.MessageEmailConfig{}).Where("guid <> ?", row.Guid).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Save(&row).Error
	}); err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *MessageEmailConfigService) SetDefault(guid string) error {
	guid = strings.TrimSpace(guid)
	if guid == "" {
		return errors.New("guid required")
	}
	now := nowMilli()
	return s.DB().Transaction(func(tx *gorm.DB) error {
		var row domains.MessageEmailConfig
		if err := tx.Where("guid = ?", guid).First(&row).Error; err != nil {
			return errors.New("email config not found")
		}
		if err := tx.Model(&domains.MessageEmailConfig{}).Where("guid <> ?", guid).Update("is_default", false).Error; err != nil {
			return err
		}
		return tx.Model(&domains.MessageEmailConfig{}).Where("guid = ?", guid).Updates(map[string]any{
			"is_default":  true,
			"status":      constants.StatusEnabled,
			"update_time": now,
		}).Error
	})
}

func (s *MessageEmailConfigService) Disable(guid string) error {
	return disableMessageEntity(s.DB(), &domains.MessageEmailConfig{}, guid)
}

func normalizeEmailConfigRequest(req SaveMessageEmailConfigRequest) SaveMessageEmailConfigRequest {
	req.Guid = strings.TrimSpace(req.Guid)
	req.Name = strings.TrimSpace(req.Name)
	req.Host = strings.TrimSpace(req.Host)
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	req.FromEmail = strings.TrimSpace(req.FromEmail)
	req.FromName = strings.TrimSpace(req.FromName)
	req.Encryption = strings.ToLower(strings.TrimSpace(req.Encryption))
	req.Remark = strings.TrimSpace(req.Remark)
	if req.Port <= 0 {
		req.Port = 465
	}
	if req.Encryption == "" {
		req.Encryption = "ssl"
	}
	if req.Status == 0 {
		req.Status = constants.StatusEnabled
	}
	return req
}
