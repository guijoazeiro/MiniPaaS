package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRateLimiterAllowsUpToLimitAndResets(t *testing.T) {
	current := time.Date(2026, time.August, 13, 20, 0, 0, 0, time.UTC)
	limiter := NewRateLimiter(2, time.Minute)
	limiter.now = func() time.Time { return current }

	if ok, _ := limiter.Allow("client"); !ok {
		t.Fatal("first request should be allowed")
	}
	if ok, _ := limiter.Allow("client"); !ok {
		t.Fatal("second request should be allowed")
	}
	if ok, retry := limiter.Allow("client"); ok || retry <= 0 {
		t.Fatalf("third request = allowed=%v retry=%s, want rejection with retry", ok, retry)
	}

	current = current.Add(time.Minute)
	if ok, _ := limiter.Allow("client"); !ok {
		t.Fatal("request should be allowed after the window resets")
	}
	if ok, _ := limiter.Allow("another-client"); !ok {
		t.Fatal("rate limit should be tracked per key")
	}
}

func TestRateLimitMiddlewareReturnsRetryAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := NewRateLimiter(1, time.Minute)
	router := gin.New()
	router.GET("/limited", limiter.RateLimit(RemoteIPKey), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/limited", nil)
		req.RemoteAddr = "192.0.2.10:1234"
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		return resp
	}

	if first := request(); first.Code != http.StatusNoContent {
		t.Fatalf("first status = %d, want %d", first.Code, http.StatusNoContent)
	}
	second := request()
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want %d", second.Code, http.StatusTooManyRequests)
	}
	if second.Header().Get("Retry-After") == "" {
		t.Fatal("rate-limited response is missing Retry-After")
	}
}

func TestRateLimiterIsSafeUnderConcurrentRequests(t *testing.T) {
	limiter := NewRateLimiter(5, time.Minute)
	var allowed atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, _ := limiter.Allow("same-client"); ok {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := allowed.Load(); got != 5 {
		t.Fatalf("allowed concurrent requests = %d, want 5", got)
	}
}

func TestRateLimiterEvictionKeepsStateBounded(t *testing.T) {
	limiter := NewRateLimiter(1, time.Minute)
	limiter.now = func() time.Time { return time.Unix(1000, 0) }
	limiter.mu.Lock()
	for i := 0; i < maxTrackedClients+500; i++ {
		limiter.clients["client-"+strconv.Itoa(i)] = rateLimitState{started: time.Unix(999, 0), count: 1}
	}
	limiter.mu.Unlock()

	if ok, _ := limiter.Allow("new-client"); !ok {
		t.Fatal("new client should be allowed")
	}
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if len(limiter.clients) > maxTrackedClients {
		t.Fatalf("tracked clients = %d, want at most %d", len(limiter.clients), maxTrackedClients)
	}
}
