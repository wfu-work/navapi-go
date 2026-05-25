package services

import (
	"sync"
	"time"
)

type RateLimitService struct {
	mu      sync.Mutex
	buckets map[string]*rateLimitBucket
}

type rateLimitBucket struct {
	WindowStart time.Time
	Count       int64
}

var RateLimitServiceApp = &RateLimitService{buckets: map[string]*rateLimitBucket{}}

func (s *RateLimitService) Allow(key string, limit int64, window time.Duration) (bool, time.Duration) {
	if limit <= 0 || window <= 0 {
		return true, 0
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	bucket := s.buckets[key]
	if bucket == nil || now.Sub(bucket.WindowStart) >= window {
		s.buckets[key] = &rateLimitBucket{WindowStart: now, Count: 1}
		s.cleanupLocked(now, window)
		return true, 0
	}
	if bucket.Count >= limit {
		return false, window - now.Sub(bucket.WindowStart)
	}
	bucket.Count++
	return true, 0
}

func (s *RateLimitService) cleanupLocked(now time.Time, window time.Duration) {
	for key, bucket := range s.buckets {
		if now.Sub(bucket.WindowStart) > window*2 {
			delete(s.buckets, key)
		}
	}
}
