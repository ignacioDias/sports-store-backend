package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sports-store/internal/database/repositories"
	"sports-store/internal/domains"
	"testing"
)

type fakeSessionStore struct {
	session *domains.Session
	err     error
}

func (f fakeSessionStore) GetSessionByID(context.Context, string) (*domains.Session, error) {
	return f.session, f.err
}

func TestAuthenticationMiddlewareAddsStringUserID(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/private", nil)
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "session"})
	var got string

	handler := NewAuthenticationMiddleware(fakeSessionStore{
		session: &domains.Session{UserID: "user-123"},
	}).AuthenticationMiddleware(func(w http.ResponseWriter, r *http.Request) {
		var ok bool
		got, ok = GetUserID(r)
		if !ok {
			t.Error("expected user ID in request context")
		}
	})

	handler(httptest.NewRecorder(), request)
	if got != "user-123" {
		t.Fatalf("got user ID %q, want %q", got, "user-123")
	}
}

func TestAuthenticationMiddlewareRejectsMissingAndInvalidSessions(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "missing cookie"},
		{name: "not found", err: errors.Join(errors.New("wrapped"), repositories.ErrSessionNotFound)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/private", nil)
			if tt.err != nil {
				request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "session"})
			}
			handler := NewAuthenticationMiddleware(fakeSessionStore{err: tt.err}).AuthenticationMiddleware(func(http.ResponseWriter, *http.Request) {
				t.Error("next handler should not be called")
			})
			response := httptest.NewRecorder()
			handler(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("got status %d, want %d", response.Code, http.StatusUnauthorized)
			}
		})
	}
}
