package services

import (
	"strconv"
	"sync"

	"navapi-go/domains"

	"github.com/wfu-work/nav-common-go-lib/global"
	"gorm.io/gorm/clause"
)

type OptionService struct {
	mu    sync.RWMutex
	cache map[string]string
}

var OptionServiceApp = &OptionService{cache: map[string]string{}}

func (s *OptionService) Load() error {
	var options []domains.Option
	if err := global.NAV_DB.Find(&options).Error; err != nil {
		return err
	}
	next := map[string]string{}
	for _, option := range options {
		next[option.Key] = option.Value
	}
	s.mu.Lock()
	s.cache = next
	s.mu.Unlock()
	return nil
}

func (s *OptionService) All() (map[string]string, error) {
	if len(s.cache) == 0 {
		if err := s.Load(); err != nil {
			return nil, err
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := map[string]string{}
	for key, value := range s.cache {
		out[key] = value
	}
	return out, nil
}

func (s *OptionService) Get(key string, fallback string) string {
	if len(s.cache) == 0 {
		_ = s.Load()
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if value, ok := s.cache[key]; ok {
		return value
	}
	return fallback
}

func (s *OptionService) Int64(key string, fallback int64) int64 {
	value := s.Get(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func (s *OptionService) Set(key string, value string) error {
	option := domains.Option{Key: key, Value: value}
	err := global.NAV_DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&option).Error
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.cache[key] = value
	s.mu.Unlock()
	return nil
}

func (s *OptionService) Delete(key string) error {
	if err := global.NAV_DB.Delete(&domains.Option{}, "key = ?", key).Error; err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.cache, key)
	s.mu.Unlock()
	return nil
}
