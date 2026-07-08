package services

import (
	"errors"
	"time"

	commonServices "github.com/wfu-work/nav-common-go-lib/services"
	"gorm.io/gorm"
	"navapi-go/domains"
	"navapi-go/vos"
)

type CheckinService struct {
	commonServices.CrudService[domains.CheckinRecord]
}

var CheckinServiceApp = new(CheckinService)

func (s *CheckinService) WithDB(db *gorm.DB) *CheckinService {
	cloned := *s
	cloned.CrudService = *s.CrudService.WithDB(db)
	return &cloned
}

type CheckinSettings struct {
	Enabled           bool  `json:"enabled"`
	DailyAmount       int64 `json:"dailyAmount"`
	StreakBonusAmount int64 `json:"streakBonusAmount"`
	MaxBonusDays      int   `json:"maxBonusDays"`
}

type CheckinRequest struct {
	TokenID uint `json:"tokenId"`
}

type CheckinStatus struct {
	TodayChecked bool   `json:"todayChecked"`
	Today        string `json:"today"`
	Streak       int    `json:"streak"`
	TodayAmount  int64  `json:"todayAmount"`
	NextAmount   int64  `json:"nextAmount"`
}

func (s *CheckinService) Settings() CheckinSettings {
	return CheckinSettings{
		Enabled:           OptionServiceApp.Int64("checkin.enabled", 1) > 0,
		DailyAmount:       OptionServiceApp.Int64("checkin.daily_amount", 0),
		StreakBonusAmount: OptionServiceApp.Int64("checkin.streak_bonus_amount", 0),
		MaxBonusDays:      int(OptionServiceApp.Int64("checkin.max_bonus_days", 7)),
	}
}

func (s *CheckinService) SetSettings(settings CheckinSettings) error {
	values := map[string]string{
		"checkin.daily_amount":        int64ToString(settings.DailyAmount),
		"checkin.streak_bonus_amount": int64ToString(settings.StreakBonusAmount),
		"checkin.max_bonus_days":      int64ToString(int64(settings.MaxBonusDays)),
	}
	if settings.Enabled {
		values["checkin.enabled"] = "1"
	} else {
		values["checkin.enabled"] = "0"
	}
	for key, value := range values {
		if err := OptionServiceApp.Set(key, value); err != nil {
			return err
		}
	}
	return nil
}

func (s *CheckinService) List(userGuid string, query vos.PageQuery) (vos.PageResult, error) {
	query.Normalize()
	var records []domains.CheckinRecord
	var total int64
	db := s.DB().Model(&domains.CheckinRecord{})
	if userGuid != "" {
		db = db.Where("user_guid = ?", userGuid)
	}
	if query.Q != "" {
		db = db.Where("user_guid LIKE ? OR date LIKE ? OR status LIKE ?", "%"+query.Q+"%", "%"+query.Q+"%", "%"+query.Q+"%")
	}
	if err := db.Count(&total).Error; err != nil {
		return vos.PageResult{}, err
	}
	if err := db.Order("date desc, id desc").Offset(query.Offset()).Limit(query.Size).Find(&records).Error; err != nil {
		return vos.PageResult{}, err
	}
	return vos.PageResult{List: records, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s *CheckinService) Status(userGuid string) (CheckinStatus, error) {
	if userGuid == "" {
		return CheckinStatus{}, errors.New("user is required")
	}
	today := todayString()
	status := CheckinStatus{Today: today}
	var record domains.CheckinRecord
	err := s.DB().Where("user_guid = ? AND date = ?", userGuid, today).First(&record).Error
	if err == nil {
		status.TodayChecked = true
		status.Streak = record.Streak
		status.TodayAmount = record.RewardAmount
		status.NextAmount = s.calculateReward(record.Streak + 1)
		return status, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return status, err
	}
	streak, err := s.previousStreak(userGuid, today)
	if err != nil {
		return status, err
	}
	status.Streak = streak
	status.NextAmount = s.calculateReward(streak + 1)
	return status, nil
}

// Checkin creates exactly one record per user/day and grants the configured
// reward in the same transaction.
func (s *CheckinService) Checkin(userGuid string, req CheckinRequest) (*domains.CheckinRecord, error) {
	if userGuid == "" {
		return nil, errors.New("user is required")
	}
	settings := s.Settings()
	if !settings.Enabled {
		return nil, errors.New("checkin is disabled")
	}
	today := todayString()
	var created domains.CheckinRecord
	err := s.DB().Transaction(func(tx *gorm.DB) error {
		var existing domains.CheckinRecord
		err := tx.Where("user_guid = ? AND date = ?", userGuid, today).First(&existing).Error
		if err == nil {
			return errors.New("already checked in today")
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		streak, err := s.previousStreak(userGuid, today)
		if err != nil {
			return err
		}
		streak++
		reward := s.calculateReward(streak)
		record := domains.CheckinRecord{
			UserGuid:     userGuid,
			Date:         today,
			RewardAmount: reward,
			Streak:       streak,
			TokenID:      req.TokenID,
			Status:       "success",
		}
		if err := record.BeforeCreate(nil); err != nil {
			return err
		}
		recordCrud := s.CrudService.WithDB(tx)
		if err := recordCrud.Create(record); err != nil {
			return err
		}
		if reward > 0 {
			amountMicros := WholeAmountToMicros(reward)
			if req.TokenID > 0 {
				if err := TokenServiceApp.AddAmount(tx, req.TokenID, userGuid, amountMicros); err != nil {
					return err
				}
			}
			if err := UserWalletServiceApp.RecordIncome(tx, WalletRecordInput{
				UserGuid:     userGuid,
				Type:         domains.WalletRecordTypeReward,
				Source:       domains.WalletSourceCheckin,
				Title:        "签到奖励",
				AmountMicros: amountMicros,
				TokenID:      req.TokenID,
				RelatedGuid:  record.Guid,
				Remark:       record.Date,
			}); err != nil {
				return err
			}
		}
		created = record
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func (s *CheckinService) previousStreak(userGuid string, today string) (int, error) {
	yesterday, err := dateBefore(today)
	if err != nil {
		return 0, err
	}
	var record domains.CheckinRecord
	err = s.DB().Where("user_guid = ? AND date = ?", userGuid, yesterday).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return record.Streak, nil
}

func (s *CheckinService) calculateReward(streak int) int64 {
	settings := s.Settings()
	if streak <= 0 {
		streak = 1
	}
	bonusDays := streak - 1
	if settings.MaxBonusDays > 0 && bonusDays > settings.MaxBonusDays {
		bonusDays = settings.MaxBonusDays
	}
	return settings.DailyAmount + int64(bonusDays)*settings.StreakBonusAmount
}

func todayString() string {
	return time.Now().Format("2006-01-02")
}

func dateBefore(date string) (string, error) {
	parsed, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return "", err
	}
	return parsed.AddDate(0, 0, -1).Format("2006-01-02"), nil
}
