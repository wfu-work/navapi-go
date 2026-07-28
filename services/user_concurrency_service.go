package services

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
)

type UserConcurrencyService struct {
	mu           sync.RWMutex
	active       map[string]int
	activeTokens map[string]int
}

var UserConcurrencyServiceApp = &UserConcurrencyService{
	active:       map[string]int{},
	activeTokens: map[string]int{},
}

func (s *UserConcurrencyService) Acquire(userGuid, tokenGuid string) (func(), error) {
	userGuid = strings.TrimSpace(userGuid)
	tokenGuid = strings.TrimSpace(tokenGuid)
	limit := 0
	if userGuid != "" {
		settings, err := UserSettingsServiceApp.Get(userGuid)
		if err != nil {
			return nil, err
		}
		limit = settings.MaxConcurrency
		if limit <= 0 {
			limit = DefaultUserMaxConcurrency
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		s.active = map[string]int{}
	}
	current := s.active[userGuid]
	if userGuid != "" && current >= limit {
		return nil, &RelayHTTPError{
			StatusCode: http.StatusTooManyRequests,
			Message:    fmt.Sprintf("user concurrency limit exceeded, max concurrency is %d", limit),
		}
	}
	if userGuid != "" {
		s.active[userGuid] = current + 1
	}
	if tokenGuid != "" {
		if s.activeTokens == nil {
			s.activeTokens = map[string]int{}
		}
		s.activeTokens[tokenGuid]++
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			s.release(userGuid, tokenGuid)
		})
	}, nil
}

func (s *UserConcurrencyService) ActiveTokenCounts(tokenGuids []string) map[string]int {
	counts := make(map[string]int, len(tokenGuids))
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, tokenGuid := range tokenGuids {
		tokenGuid = strings.TrimSpace(tokenGuid)
		if tokenGuid == "" {
			continue
		}
		counts[tokenGuid] = s.activeTokens[tokenGuid]
	}
	return counts
}

func (s *UserConcurrencyService) release(userGuid, tokenGuid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if userGuid != "" {
		decrementConcurrency(s.active, userGuid)
	}
	if tokenGuid != "" {
		decrementConcurrency(s.activeTokens, tokenGuid)
	}
}

func decrementConcurrency(active map[string]int, key string) {
	if active[key] <= 1 {
		delete(active, key)
		return
	}
	active[key]--
}

func (s *UserConcurrencyService) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = map[string]int{}
	s.activeTokens = map[string]int{}
}
