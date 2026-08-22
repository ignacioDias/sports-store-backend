package middleware

import (
	"net/http"
	"time"
)

func NewSessionCookie(value string, secure bool, expires time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    value,
		Path:     "/",
		Expires:  expires,
		MaxAge:   maxAge(expires),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

func maxAge(expires time.Time) int {
	seconds := int(time.Until(expires).Seconds())
	if seconds < 0 {
		return 0
	}
	return seconds
}
