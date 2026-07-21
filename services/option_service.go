package services

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"navapi-go/domains"

	"github.com/wfu-work/nav-common-go-lib/global"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OptionService struct {
	opMu  sync.Mutex
	mu    sync.RWMutex
	cache map[string]string
}

var OptionServiceApp = &OptionService{cache: map[string]string{}}

func (s *OptionService) Load() error {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if global.NAV_DB == nil {
		return nil
	}
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

func (s *OptionService) Bool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(s.Get(key, "")))
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func (s *OptionService) Set(key string, value string) error {
	return s.SetMany(map[string]string{key: value})
}

// SetMany persists a related configuration set atomically, then publishes the
// same values to the in-memory cache only after the transaction commits.
func (s *OptionService) SetMany(values map[string]string) error {
	if len(values) == 0 {
		return nil
	}
	if global.NAV_DB == nil {
		return errors.New("database is not initialized")
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	keys := make([]string, 0, len(values))
	for key := range values {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			return errors.New("option key is required")
		}
		if trimmedKey != key {
			return fmt.Errorf("option key %q must not contain surrounding whitespace", key)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if err := global.NAV_DB.Transaction(func(tx *gorm.DB) error {
		for _, key := range keys {
			option := domains.Option{Key: key, Value: values[key]}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "key"}},
				DoUpdates: clause.AssignmentColumns([]string{"value"}),
			}).Create(&option).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	s.mu.Lock()
	if s.cache == nil {
		s.cache = map[string]string{}
	}
	for _, key := range keys {
		s.cache[key] = values[key]
	}
	s.mu.Unlock()
	return nil
}

func (s *OptionService) Delete(key string) error {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if err := global.NAV_DB.Delete(&domains.Option{}, "key = ?", key).Error; err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.cache, key)
	s.mu.Unlock()
	return nil
}
