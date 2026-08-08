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

const oidcStateCookieName = "heliobot_oidc_state"

// OIDCState holds the state and PKCE verifier for an authorization request.
type OIDCState struct {
	State    string `json:"s"`
	Verifier string `json:"v"`
	Expires  int64  `json:"exp"`
}

// OIDCStateManager issues and verifies signed OIDC state cookies.
type OIDCStateManager struct {
	secret   []byte
	maxAge   time.Duration
	secure   bool
	sameSite http.SameSite
	domain   string
}

// NewOIDCStateManager creates an OIDC state cookie manager.
func NewOIDCStateManager(secret string, maxAgeSeconds int, secure bool, sameSite string, domain string) *OIDCStateManager {
	ss := http.SameSiteLaxMode
	switch strings.ToLower(sameSite) {
	case "strict":
		ss = http.SameSiteStrictMode
	case "none":
		ss = http.SameSiteNoneMode
	case "lax", "":
		ss = http.SameSiteLaxMode
	}
	return &OIDCStateManager{
		secret: []byte(secret),
		maxAge: time.Duration(maxAgeSeconds) * time.Second,
		secure: secure,
		sameSite: ss,
		domain:   domain,
	}
}

// IssueCookie stores the OIDC state in a signed HTTP-only cookie.
func (sm *OIDCStateManager) IssueCookie(w http.ResponseWriter, state *OIDCState) error {
	state.Expires = time.Now().Add(sm.maxAge).Unix()
	token, err := sm.sign(state)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookieName,
		Value:    token,
		Path:     "/api/auth/telegram/oidc",
		Domain:   sm.domain,
		MaxAge:   int(sm.maxAge.Seconds()),
		HttpOnly: true,
		Secure:   sm.secure,
		SameSite: sm.sameSite,
	})
	return nil
}

// ClearCookie removes the OIDC state cookie.
func (sm *OIDCStateManager) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookieName,
		Value:    "",
		Path:     "/api/auth/telegram/oidc",
		Domain:   sm.domain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   sm.secure,
		SameSite: sm.sameSite,
	})
}

// FromRequest extracts and verifies the OIDC state from the request.
func (sm *OIDCStateManager) FromRequest(r *http.Request) (*OIDCState, error) {
	cookie, err := r.Cookie(oidcStateCookieName)
	if err != nil {
		return nil, err
	}
	return sm.verify(cookie.Value)
}

func (sm *OIDCStateManager) sign(state *OIDCState) (string, error) {
	b, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("marshal oidc state: %w", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(b)
	sig := hmac.New(sha256.New, sm.secret)
	sig.Write([]byte(payload))
	signature := base64.RawURLEncoding.EncodeToString(sig.Sum(nil))
	return payload + "." + signature, nil
}

func (sm *OIDCStateManager) verify(token string) (*OIDCState, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, errors.New("invalid oidc state format")
	}
	sig := hmac.New(sha256.New, sm.secret)
	sig.Write([]byte(parts[0]))
	expectedSig := base64.RawURLEncoding.EncodeToString(sig.Sum(nil))
	if !hmac.Equal([]byte(expectedSig), []byte(parts[1])) {
		return nil, errors.New("invalid oidc state signature")
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decode oidc state: %w", err)
	}
	var state OIDCState
	if err := json.Unmarshal(b, &state); err != nil {
		return nil, fmt.Errorf("unmarshal oidc state: %w", err)
	}
	if time.Now().Unix() > state.Expires {
		return nil, errors.New("oidc state expired")
	}
	return &state, nil
}
