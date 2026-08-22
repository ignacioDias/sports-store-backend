package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type RateLimitMiddleware struct {
	ipRateLimiter RateLimiterStore
	clientIP      ClientIPResolver
}

type RateLimiterStore interface {
	GetLimiter(string) Limiter
}

type Limiter interface {
	Allow() bool
}

type ClientIPResolver interface {
	ClientIP(*http.Request) (string, error)
}

type RemoteAddrResolver struct{}

func (RemoteAddrResolver) ClientIP(r *http.Request) (string, error) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", err
	}
	if net.ParseIP(ip) == nil {
		return "", net.InvalidAddrError(ip)
	}
	return ip, nil
}

type IPRateLimiter struct {
	limiters        map[string]*RateLimiter
	mutex           sync.Mutex
	lastCleanupTime time.Time
}

const (
	limiterCleanupInterval = time.Minute
	maxTrackedIPs          = 10000
)

type RateLimiter struct {
	tokens         float64   // Current number of tokens
	maxTokens      float64   // Maximum tokens allowed
	refillRate     float64   // Tokens added per second
	lastRefillTime time.Time // Last time tokens were refilled
	lastAccessTime time.Time // Last time this limiter was accessed
	mutex          sync.Mutex
}

func NewRateLimiter(maxTokens, refillRate float64) *RateLimiter {
	now := time.Now()
	return &RateLimiter{
		tokens:         maxTokens,
		maxTokens:      maxTokens,
		refillRate:     refillRate,
		lastRefillTime: now,
		lastAccessTime: now,
	}
}
func NewIPRateLimiter() *IPRateLimiter {
	return &IPRateLimiter{
		limiters:        make(map[string]*RateLimiter),
		lastCleanupTime: time.Now(),
	}
}

func NewRateLimitMiddleware(store RateLimiterStore, resolver ClientIPResolver) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		ipRateLimiter: store,
		clientIP:      resolver,
	}
}

func NewDefaultRateLimitMiddleware() *RateLimitMiddleware {
	return NewRateLimitMiddleware(NewIPRateLimiter(), RemoteAddrResolver{})
}

func (i *IPRateLimiter) GetLimiter(ip string) Limiter {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	if time.Since(i.lastCleanupTime) >= limiterCleanupInterval {
		i.cleanupOldLimiters()
		i.lastCleanupTime = time.Now()
	}

	limiter, exists := i.limiters[ip]
	if !exists {
		if len(i.limiters) >= maxTrackedIPs {
			i.evictOldestLimiter()
		}
		// Allow 3 requests per minute
		limiter = NewRateLimiter(3, 0.05)
		i.limiters[ip] = limiter
	} else {
		// Update last access time
		limiter.lastAccessTime = time.Now()
	}

	return limiter
}

func (i *IPRateLimiter) evictOldestLimiter() {
	var oldestIP string
	var oldest time.Time
	for ip, limiter := range i.limiters {
		if oldestIP == "" || limiter.lastAccessTime.Before(oldest) {
			oldestIP = ip
			oldest = limiter.lastAccessTime
		}
	}
	if oldestIP != "" {
		delete(i.limiters, oldestIP)
	}
}

// cleanupOldLimiters removes entries not accessed in the last 24 hours (call while holding mutex)
func (i *IPRateLimiter) cleanupOldLimiters() {
	cutoff := time.Now().Add(-24 * time.Hour)
	for ip, limiter := range i.limiters {
		if limiter.lastAccessTime.Before(cutoff) {
			delete(i.limiters, ip)
		}
	}
}
func (r *RateLimiter) Allow() bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.refillTokens()

	if r.tokens >= 1 {
		r.tokens--
		return true
	}
	return false
}

func (r *RateLimiter) refillTokens() {
	now := time.Now()
	duration := now.Sub(r.lastRefillTime).Seconds()
	tokensToAdd := duration * r.refillRate

	r.tokens += tokensToAdd
	if r.tokens > r.maxTokens {
		r.tokens = r.maxTokens
	}
	r.lastRefillTime = now
}

func (rateLimitMiddleware *RateLimitMiddleware) RateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip, err := rateLimitMiddleware.clientIP.ClientIP(r)
		if err != nil || ip == "" {
			http.Error(w, "Invalid client address", http.StatusBadRequest)
			return
		}

		limiter := rateLimitMiddleware.ipRateLimiter.GetLimiter(ip)
		if limiter != nil && limiter.Allow() {
			next(w, r)
		} else {
			w.Header().Set("Retry-After", "20")
			http.Error(w, "Rate Limit Exceeded", http.StatusTooManyRequests)
		}
	}
}
