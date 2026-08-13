package middleware

import (
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter is a small fixed-window limiter intended for single-process
// protection of sensitive endpoints. A distributed deployment should replace
// it with a shared store before running multiple API instances.
type RateLimiter struct {
	limit  int
	window time.Duration
	now    func() time.Time

	mu      sync.Mutex
	clients map[string]rateLimitState
}

type rateLimitState struct {
	started time.Time
	count   int
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	if limit < 1 {
		limit = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	return &RateLimiter{
		limit:   limit,
		window:  window,
		now:     time.Now,
		clients: make(map[string]rateLimitState),
	}
}

// Allow consumes one request for key and returns the retry delay when the
// current window is exhausted.
func (l *RateLimiter) Allow(key string) (allowed bool, retryAfter time.Duration) {
	if l == nil {
		return true, 0
	}
	key = strings.TrimSpace(key)
	if key == "" {
		key = "unknown"
	}
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()
	if state, ok := l.clients[key]; ok && now.Sub(state.started) >= 0 && now.Sub(state.started) < l.window {
		if state.count >= l.limit {
			return false, l.window - now.Sub(state.started)
		}
		state.count++
		l.clients[key] = state
		return true, 0
	}

	l.clients[key] = rateLimitState{started: now, count: 1}
	// Bound state growth when a process receives many distinct clients. Prefer
	// removing expired windows, then evict the oldest active entries.
	for len(l.clients) > 1024 {
		oldestKey := ""
		oldest := now
		for client, state := range l.clients {
			if now.Sub(state.started) >= l.window {
				delete(l.clients, client)
				continue
			}
			if oldestKey == "" || state.started.Before(oldest) {
				oldestKey = client
				oldest = state.started
			}
		}
		if oldestKey == "" {
			break
		}
		delete(l.clients, oldestKey)
	}
	return true, 0
}

// RateLimit returns a Gin middleware that responds with 429 after the limit
// is reached. The key function should identify the caller, normally by IP.
func (l *RateLimiter) RateLimit(key func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		caller := "unknown"
		if key != nil {
			caller = key(c)
		}
		allowed, retryAfter := l.Allow(caller)
		if !allowed {
			seconds := int((retryAfter + time.Second - 1) / time.Second)
			if seconds < 1 {
				seconds = 1
			}
			c.Header("Retry-After", strconv.Itoa(seconds))
			c.AbortWithStatusJSON(429, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}

// RemoteIPKey avoids trusting arbitrary X-Forwarded-For headers. When the API
// is placed behind a trusted proxy, configure that proxy to preserve a stable
// source address or replace this key function with a trusted-proxy variant.
func RemoteIPKey(c *gin.Context) string {
	remote := c.Request.RemoteAddr
	if host, _, err := net.SplitHostPort(remote); err == nil {
		return host
	}
	return remote
}
