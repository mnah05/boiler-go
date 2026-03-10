package middleware

import (
	"net/http"
	"strings"
	"sync"

	"golang.org/x/time/rate"
)

type IPRateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

func NewIPRateLimiter(rps rate.Limit, burst int) *IPRateLimiter {
	return &IPRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rps,
		burst:    burst,
	}
}

func (r *IPRateLimiter) getLimiter(ip string) *rate.Limiter {
	r.mu.RLock()
	limiter, exists := r.limiters[ip]
	r.mu.RUnlock()

	if exists {
		return limiter
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if limiter, exists = r.limiters[ip]; exists {
		return limiter
	}

	limiter = rate.NewLimiter(r.rate, r.burst)
	r.limiters[ip] = limiter
	return limiter
}

func (r *IPRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ip := getClientIP(req)

		limiter := r.getLimiter(ip)
		if !limiter.Allow() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			http.Error(w, `{"error":"rate limit exceeded","message":"too many requests"}`, http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, req)
	})
}

func getClientIP(req *http.Request) string {
	xff := req.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	xri := req.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	if addr := req.RemoteAddr; addr != "" {
		if idx := strings.LastIndex(addr, ":"); idx != -1 {
			return addr[:idx]
		}
		return addr
	}

	return "unknown"
}

func RateLimiter(rps float64, burst int) func(next http.Handler) http.Handler {
	limiter := NewIPRateLimiter(rate.Limit(rps), burst)
	return limiter.Middleware
}

func NewDefaultRateLimiter() func(next http.Handler) http.Handler {
	return RateLimiter(10, 20)
}

func NewRateLimiterFromConfig(maxRate int, burst int) func(next http.Handler) http.Handler {
	if maxRate <= 0 {
		maxRate = 10
	}
	if burst <= 0 {
		burst = 20
	}
	return RateLimiter(float64(maxRate), burst)
}
