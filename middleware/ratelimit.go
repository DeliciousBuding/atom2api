package middleware

import (
	"net/http"
	"sync"
	"time"
)

type RateLimiter struct {
	mu       sync.Mutex
	rpm      int
	requests []time.Time
}

func NewRateLimiter(rpm int) *RateLimiter {
	return &RateLimiter{rpm: rpm}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	if rl.rpm <= 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rl.mu.Lock()
		now := time.Now()
		cutoff := now.Add(-time.Minute)
		filtered := rl.requests[:0]
		for _, t := range rl.requests {
			if t.After(cutoff) {
				filtered = append(filtered, t)
			}
		}
		rl.requests = filtered
		if len(rl.requests) >= rl.rpm {
			rl.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(429)
			w.Write([]byte(`{"error":{"message":"rate limit exceeded","type":"rate_limit_error","code":"rate_limit_exceeded"}}`))
			return
		}
		rl.requests = append(rl.requests, now)
		rl.mu.Unlock()
		next.ServeHTTP(w, r)
	})
}
