package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOIDCStateManagerCookieAttributes(t *testing.T) {
	// OIDC state cookie must be SameSite=None and Secure to survive the
	// cross-site redirect from Telegram back to the callback.
	mgr := NewOIDCStateManager("secret", 600, true, "none", "")

	rec := httptest.NewRecorder()
	if err := mgr.IssueCookie(rec, &OIDCState{State: "state", Verifier: "verifier"}); err != nil {
		t.Fatalf("issue cookie: %v", err)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected a state cookie")
	}
	c := cookies[0]
	if !c.Secure {
		t.Error("expected Secure=true")
	}
	if c.SameSite != http.SameSiteNoneMode {
		t.Errorf("expected SameSite=None, got %v", c.SameSite)
	}
	if !strings.HasPrefix(c.Path, "/api/auth/telegram/oidc") {
		t.Errorf("unexpected path: %s", c.Path)
	}
}
