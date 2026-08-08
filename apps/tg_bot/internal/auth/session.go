package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const sessionCookieName = "heliobot_session"

// Session represents an authenticated dashboard session.
type Session struct {
	UserID    int64  `json:"uid"`
	Username  string `json:"usr,omitempty"`
	FirstName string `json:"fnm,omitempty"`
	LastName  string `json:"lnm,omitempty"`
	PhotoURL  string `json:"pic,omitempty"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

// SessionManager issues and validates signed session cookies.
type SessionManager struct {
	secret     []byte
	maxAge     time.Duration
	secure     bool
	sameSite   http.SameSite
	domain     string
}

// NewSessionManager creates a session manager with the given secret and settings.
func NewSessionManager(secret string, maxAgeSeconds int, secure bool, sameSite string, domain string) *SessionManager {
	ss := http.SameSiteLaxMode
	switch strings.ToLower(sameSite) {
	case "strict":
		ss = http.SameSiteStrictMode
	case "none":
		ss = http.SameSiteNoneMode
	case "lax", "":
		ss = http.SameSiteLaxMode
	}

	return &SessionManager{
		secret:   []byte(secret),
		maxAge:   time.Duration(maxAgeSeconds) * time.Second,
		secure:   secure,
		sameSite: ss,
		domain:   domain,
	}
}

// IssueCookie creates a session from a Telegram user and sets it as an HTTP-only cookie.
func (sm *SessionManager) IssueCookie(w http.ResponseWriter, user *TelegramUser) error {
	now := time.Now()
	session := &Session{
		UserID:    user.ID,
		Username:  user.Username,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		PhotoURL:  user.PhotoURL,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(sm.maxAge).Unix(),
	}

	token, err := sm.sign(session)
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Domain:   sm.domain,
		MaxAge:   int(sm.maxAge.Seconds()),
		HttpOnly: true,
		Secure:   sm.secure,
		SameSite: sm.sameSite,
	})
	return nil
}

// ClearCookie removes the session cookie.
func (sm *SessionManager) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		Domain:   sm.domain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   sm.secure,
		SameSite: sm.sameSite,
	})
}

// FromRequest extracts and validates the session from an HTTP request.
func (sm *SessionManager) FromRequest(r *http.Request) (*Session, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, err
	}
	return sm.verify(cookie.Value)
}

// sign serializes the session and returns a signed token.
func (sm *SessionManager) sign(session *Session) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

	payloadBytes, err := json.Marshal(session)
	if err != nil {
		return "", fmt.Errorf("marshal session: %w", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)

	signingInput := header + "." + payload
	sig := hmac.New(sha256.New, sm.secret)
	sig.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(sig.Sum(nil))

	return signingInput + "." + signature, nil
}

// verify parses and verifies a signed token.
func (sm *SessionManager) verify(token string) (*Session, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	sig := hmac.New(sha256.New, sm.secret)
	sig.Write([]byte(parts[0] + "." + parts[1]))
	expectedSig := base64.RawURLEncoding.EncodeToString(sig.Sum(nil))
	if !hmac.Equal([]byte(expectedSig), []byte(parts[2])) {
		return nil, errors.New("invalid token signature")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}

	var session Session
	if err := json.Unmarshal(payloadBytes, &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	if time.Now().Unix() > session.ExpiresAt {
		return nil, errors.New("session expired")
	}

	return &session, nil
}
