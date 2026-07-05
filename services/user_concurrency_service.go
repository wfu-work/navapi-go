package services

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
)

type UserConcurrencyService struct {
	mu     sync.Mutex
	active map[string]int
}

var UserConcurrencyServiceApp = &UserConcurrencyService{active: map[string]int{}}

func (s *UserConcurrencyService) Acquire(userGuid string) (func(), error) {
	userGuid = strings.TrimSpace(userGuid)
	if userGuid == "" {
		return func() {}, nil
	}
	settings, err := UserSettingsServiceApp.Get(userGuid)
	if err != nil {
		return nil, err
	}
	limit := settings.MaxConcurrency
	if limit <= 0 {
		limit = DefaultUserMaxConcurrency
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		s.active = map[string]int{}
	}
	current := s.active[userGuid]
	if current >= limit {
		return nil, &RelayHTTPError{
			StatusCode: http.StatusTooManyRequests,
			Message:    fmt.Sprintf("user concurrency limit exceeded, max concurrency is %d", limit),
		}
	}
	s.active[userGuid] = current + 1

	var once sync.Once
	return func() {
		once.Do(func() {
			s.release(userGuid)
		})
	}, nil
}

func (s *UserConcurrencyService) release(userGuid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current := s.active[userGuid]
	if current <= 1 {
		delete(s.active, userGuid)
		return
	}
	s.active[userGuid] = current - 1
}

func (s *UserConcurrencyService) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = map[string]int{}
}
