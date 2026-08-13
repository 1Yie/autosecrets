package identity

import (
	"sync"
	"time"
)

const (
	authFailureLimit  = 5
	authFailureWindow = 5 * time.Minute
	maxFailureBuckets = 10_000
)

type failureBucket struct {
	count       int
	windowStart time.Time
}

// authFailureLimiter bounds failed authentication attempts per client source.
// Buckets expire without locking an Administrator out permanently.
type authFailureLimiter struct {
	mu      sync.Mutex
	buckets map[string]failureBucket
}

func (l *authFailureLimiter) limited(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	bucket, ok := l.buckets[key]
	if !ok || !now.Before(bucket.windowStart.Add(authFailureWindow)) {
		delete(l.buckets, key)
		return false, 0
	}
	if bucket.count < authFailureLimit {
		return false, 0
	}
	return true, bucket.windowStart.Add(authFailureWindow).Sub(now)
}

func (l *authFailureLimiter) failed(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.buckets == nil {
		l.buckets = make(map[string]failureBucket)
	}
	bucket, ok := l.buckets[key]
	if !ok || !now.Before(bucket.windowStart.Add(authFailureWindow)) {
		if len(l.buckets) >= maxFailureBuckets {
			for candidate, value := range l.buckets {
				if !now.Before(value.windowStart.Add(authFailureWindow)) {
					delete(l.buckets, candidate)
				}
			}
		}
		if len(l.buckets) >= maxFailureBuckets {
			for candidate := range l.buckets {
				delete(l.buckets, candidate)
				break
			}
		}
		bucket = failureBucket{windowStart: now}
	}
	bucket.count++
	l.buckets[key] = bucket
}

func (l *authFailureLimiter) succeeded(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.buckets, key)
}
