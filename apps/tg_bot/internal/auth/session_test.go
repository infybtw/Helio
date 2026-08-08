package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionManagerIssueAndVerify(t *testing.T) {
	sm := NewSessionManager("super-secret-key", 3600, false, "lax", "")

	rec := httptest.NewRecorder()
	user := &TelegramUser{
		ID:        42,
		FirstName: "Alice",
		Username:  "alice",
		AuthDate:  time.Now().Unix(),
	}
	if err := sm.IssueCookie(rec, user); err != nil {
		t.Fatalf("issue cookie: %v", err)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected a session cookie")
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookies[0])

	session, err := sm.FromRequest(req)
	if err != nil {
		t.Fatalf("verify cookie: %v", err)
	}
	if session.UserID != 42 {
		t.Errorf("expected user id 42, got %d", session.UserID)
	}
	if session.Username != "alice" {
		t.Errorf("expected username alice, got %s", session.Username)
	}
}

func TestSessionManagerExpired(t *testing.T) {
	sm := NewSessionManager("super-secret-key", -1, false, "lax", "")

	rec := httptest.NewRecorder()
	user := &TelegramUser{ID: 1, FirstName: "Bob", AuthDate: time.Now().Unix()}
	if err := sm.IssueCookie(rec, user); err != nil {
		t.Fatalf("issue cookie: %v", err)
	}

	cookies := rec.Result().Cookies()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookies[0])

	if _, err := sm.FromRequest(req); err == nil {
		t.Fatal("expected error for expired session")
	}
}

func TestSessionManagerTampered(t *testing.T) {
	sm := NewSessionManager("super-secret-key", 3600, false, "lax", "")

	rec := httptest.NewRecorder()
	user := &TelegramUser{ID: 1, FirstName: "Charlie", AuthDate: time.Now().Unix()}
	if err := sm.IssueCookie(rec, user); err != nil {
		t.Fatalf("issue cookie: %v", err)
	}

	cookies := rec.Result().Cookies()
	cookies[0].Value = cookies[0].Value + "tampered"

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookies[0])

	if _, err := sm.FromRequest(req); err == nil {
		t.Fatal("expected error for tampered session")
	}
}
