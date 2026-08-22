package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeLimiter struct{ allowed bool }

func (f fakeLimiter) Allow() bool { return f.allowed }

type fakeLimiterStore struct{ limiter Limiter }

func (f fakeLimiterStore) GetLimiter(string) Limiter { return f.limiter }

type fakeIPResolver struct {
	ip  string
	err error
}

func (f fakeIPResolver) ClientIP(*http.Request) (string, error) { return f.ip, f.err }

func TestRateLimitMiddlewareAllowsRequest(t *testing.T) {
	nextCalled := false
	middleware := NewRateLimitMiddleware(
		fakeLimiterStore{limiter: fakeLimiter{allowed: true}},
		fakeIPResolver{ip: "127.0.0.1"},
	)
	handler := middleware.RateLimit(func(http.ResponseWriter, *http.Request) { nextCalled = true })
	handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if !nextCalled {
		t.Error("expected next handler to be called")
	}
}

func TestRateLimitMiddlewareReturnsRetryAfterWhenBlocked(t *testing.T) {
	middleware := NewRateLimitMiddleware(
		fakeLimiterStore{limiter: fakeLimiter{}},
		fakeIPResolver{ip: "127.0.0.1"},
	)
	response := httptest.NewRecorder()
	middleware.RateLimit(func(http.ResponseWriter, *http.Request) {
		t.Error("next handler should not be called")
	})(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") == "" {
		t.Fatalf("got status %d and Retry-After %q", response.Code, response.Header().Get("Retry-After"))
	}
}
