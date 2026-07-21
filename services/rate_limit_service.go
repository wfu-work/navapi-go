package services

import (
	"sync"
	"time"
)

type RateLimitService struct {
	mu      sync.Mutex
	buckets map[string]*rateLimitBucket
	now     func() time.Time
}

type rateLimitBucket struct {
	ExpiresAt time.Time
	Count     int64
}

var RateLimitServiceApp = &RateLimitService{buckets: map[string]*rateLimitBucket{}, now: time.Now}

func (s *RateLimitService) Allow(key string, limit int64, window time.Duration) (bool, time.Duration) {
	if limit <= 0 || window <= 0 {
		return true, 0
	}
	now := time.Now()
	if s.now != nil {
		now = s.now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	bucket := s.buckets[key]
	if bucket == nil || !now.Before(bucket.ExpiresAt) {
		s.buckets[key] = &rateLimitBucket{ExpiresAt: now.Add(window), Count: 1}
		s.cleanupLocked(now)
		return true, 0
	}
	if bucket.Count >= limit {
		return false, bucket.ExpiresAt.Sub(now)
	}
	bucket.Count++
	return true, 0
}

func (s *RateLimitService) Reset() {
	s.mu.Lock()
	s.buckets = map[string]*rateLimitBucket{}
	s.mu.Unlock()
}

func (s *RateLimitService) cleanupLocked(now time.Time) {
	for key, bucket := range s.buckets {
		if bucket == nil || !now.Before(bucket.ExpiresAt) {
			delete(s.buckets, key)
		}
	}
}
