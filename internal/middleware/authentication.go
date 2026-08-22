package middleware

import (
	"context"
	"errors"
	"net/http"
	"sports-store/internal/database/repositories"
	"sports-store/internal/domains"
)

type contextKey string

type SessionStore interface {
	GetSessionByID(context.Context, string) (*domains.Session, error)
}

type AuthenticationMiddleware struct {
	sessionRepo SessionStore
}

const userIDKey contextKey = "userID"
const SessionCookieName = "session_id"

func NewAuthenticationMiddleware(sessionRepo SessionStore) *AuthenticationMiddleware {
	return &AuthenticationMiddleware{
		sessionRepo: sessionRepo,
	}
}

func (auth *AuthenticationMiddleware) AuthenticationMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil || cookie.Value == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		session, err := auth.sessionRepo.GetSessionByID(r.Context(), cookie.Value)
		if errors.Is(err, repositories.ErrSessionNotFound) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if err != nil {
			http.Error(w, "Server error", http.StatusInternalServerError)
			return
		}
		if session == nil || session.UserID == "" {
			http.Error(w, "Server error", http.StatusInternalServerError)
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, session.UserID)
		r = r.WithContext(ctx)

		next(w, r)
	}
}

func GetUserID(r *http.Request) (string, bool) {
	userID, ok := r.Context().Value(userIDKey).(string)
	return userID, ok
}
