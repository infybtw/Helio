package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"tg_bot/internal/auth"
	"tg_bot/internal/database"
	"tg_bot/internal/telegram"
)

// Auth handles dashboard authentication via Telegram OIDC.
type Auth struct {
	oidc       *auth.OIDCClient
	stateMgr   *auth.OIDCStateManager
	sessions   *auth.SessionManager
	db         database.Store
	client     *telegram.Client
	dashOrigin string
	dashURL    string
	log        *slog.Logger
}

// NewAuth creates a new auth handler.
func NewAuth(oidc *auth.OIDCClient, stateMgr *auth.OIDCStateManager, sessions *auth.SessionManager, db database.Store, client *telegram.Client, dashboardOrigin, dashboardURL string, log *slog.Logger) *Auth {
	return &Auth{
		oidc:       oidc,
		stateMgr:   stateMgr,
		sessions:   sessions,
		db:         db,
		client:     client,
		dashOrigin: strings.TrimSuffix(dashboardOrigin, "/"),
		dashURL:    dashboardURL,
		log:        log,
	}
}

// OIDCAuthorize initiates the Telegram OIDC authorization code flow.
func (a *Auth) OIDCAuthorize(c *gin.Context) {
	state, err := auth.GenerateState()
	if err != nil {
		a.log.Error("failed to generate oidc state", "error", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to initiate login"})
		return
	}

	pkce, err := auth.GeneratePKCE()
	if err != nil {
		a.log.Error("failed to generate pkce", "error", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to initiate login"})
		return
	}

	if err := a.stateMgr.IssueCookie(c.Writer, &auth.OIDCState{
		State:    state,
		Verifier: pkce.Verifier,
	}); err != nil {
		a.log.Error("failed to issue oidc state cookie", "error", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to initiate login"})
		return
	}

	c.Redirect(http.StatusFound, a.oidc.AuthorizationURL(state, pkce))
}

// OIDCCallback handles the Telegram OIDC redirect and completes authentication.
func (a *Auth) OIDCCallback(c *gin.Context) {
	state, err := a.stateMgr.FromRequest(c.Request)
	if err != nil {
		a.log.Warn("missing or invalid oidc state cookie", "error", err, "remote_ip", c.ClientIP())
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid or expired login session"})
		return
	}
	defer a.stateMgr.ClearCookie(c.Writer)

	if c.Query("state") != state.State {
		a.log.Warn("oidc state mismatch", "remote_ip", c.ClientIP())
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "state mismatch"})
		return
	}

	code := c.Query("code")
	if code == "" {
		a.log.Warn("missing authorization code", "remote_ip", c.ClientIP())
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "missing authorization code"})
		return
	}

	tokens, err := a.oidc.ExchangeCode(code, state.Verifier)
	if err != nil {
		a.log.Warn("failed to exchange oidc code", "error", err, "remote_ip", c.ClientIP())
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "failed to exchange authorization code"})
		return
	}

	user, err := a.oidc.ValidateIDToken(tokens.IDToken)
	if err != nil {
		a.log.Warn("failed to validate id token",
			"error", err,
			"remote_ip", c.ClientIP(),
			"id_token_length", len(tokens.IDToken),
		)
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid id token", "details": err.Error()})
		return
	}

	isOwner, err := a.isTrackedChatOwner(c.Request.Context(), user.ID)
	if err != nil {
		a.log.Error("failed to verify dashboard authorization", "error", err, "user_id", user.ID)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to verify dashboard authorization"})
		return
	}
	if !isOwner {
		a.log.Warn("telegram user is not a group owner", "user_id", user.ID)
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "dashboard access requires group ownership"})
		return
	}

	if err := a.sessions.IssueCookie(c.Writer, &auth.TelegramUser{
		ID:        user.ID,
		FirstName: user.GivenName,
		LastName:  user.FamilyName,
		Username:  user.Username,
		PhotoURL:  user.Picture,
	}); err != nil {
		a.log.Error("failed to issue session cookie", "error", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}

	a.log.Info("oidc auth succeeded", "user_id", user.ID, "username", user.Username)

	redirect := a.safeRedirect(c.Query("redirect"))
	callbackURL := a.dashOrigin + "/auth/callback?redirect=" + url.QueryEscape(redirect)
	c.Redirect(http.StatusFound, callbackURL)
}

func (a *Auth) isTrackedChatOwner(parent context.Context, userID int64) (bool, error) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()

	chatIDs, err := a.db.ListTrackedChatIDs(ctx)
	if err != nil {
		return false, err
	}
	for _, chatID := range chatIDs {
		member, err := a.client.GetChatMember(ctx, chatID, userID)
		if err != nil {
			continue
		}
		if member.Status == "creator" {
			return true, nil
		}
		if err := ctx.Err(); err != nil {
			return false, err
		}
	}
	return false, nil
}

// Logout clears the session cookie.
func (a *Auth) Logout(c *gin.Context) {
	a.sessions.ClearCookie(c.Writer)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// Me returns the currently authenticated user.
func (a *Auth) Me(c *gin.Context) {
	session, err := a.sessions.FromRequest(c.Request)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if ok, err := a.isTrackedChatOwner(c.Request.Context(), session.UserID); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to verify dashboard authorization"})
		return
	} else if !ok {
		a.sessions.ClearCookie(c.Writer)
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "dashboard access requires group ownership"})
		return
	}
	c.JSON(http.StatusOK, session)
}

// DashboardOverview returns moderation metrics for chats visible to the user.
func (a *Auth) DashboardOverview(c *gin.Context) {
	chatIDs, err := a.db.ListTrackedChatIDs(c.Request.Context())
	if err != nil {
		a.log.Error("failed to list dashboard chats", "error", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to load dashboard data"})
		return
	}
	session := SessionFromContext(c)
	ownedChatIDs := make([]int64, 0, len(chatIDs))
	for _, chatID := range chatIDs {
		member, err := a.client.GetChatMember(c.Request.Context(), chatID, session.UserID)
		if err == nil && member.Status == "creator" {
			ownedChatIDs = append(ownedChatIDs, chatID)
		}
	}
	data, err := a.db.DashboardData(c.Request.Context(), ownedChatIDs)
	if err != nil {
		a.log.Error("failed to load dashboard data", "error", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to load dashboard data"})
		return
	}
	for i := range data.Chats {
		members, err := a.client.GetChatMemberCount(c.Request.Context(), data.Chats[i].ChatID)
		if err != nil {
			a.log.Warn("failed to get dashboard chat member count", "error", err, "chat_id", data.Chats[i].ChatID)
			continue
		}
		data.Chats[i].Members = members
	}
	c.JSON(http.StatusOK, data)
}

// RequireAuth is a Gin middleware that ensures a valid session exists.
func (a *Auth) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		session, err := a.sessions.FromRequest(c.Request)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		if ok, err := a.isTrackedChatOwner(c.Request.Context(), session.UserID); err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to verify dashboard authorization"})
			return
		} else if !ok {
			a.sessions.ClearCookie(c.Writer)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "dashboard access requires group ownership"})
			return
		}
		c.Set("session", session)
		c.Next()
	}
}

// SessionFromContext retrieves the validated session from the Gin context.
func SessionFromContext(c *gin.Context) *auth.Session {
	session, ok := c.Get("session")
	if !ok {
		return nil
	}
	s, ok := session.(*auth.Session)
	if !ok {
		return nil
	}
	return s
}

// safeRedirect ensures the redirect target stays within the dashboard.
func (a *Auth) safeRedirect(target string) string {
	if target == "" {
		return a.dashURL
	}
	u, err := url.Parse(target)
	if err != nil {
		return a.dashURL
	}
	if u.IsAbs() {
		return a.dashURL
	}
	return target
}

// AuthConfigJSON returns the configuration the frontend needs to render the login link.
func (a *Auth) AuthConfigJSON(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"authorize_url": "/api/auth/telegram/oidc/authorize?redirect=/dashboard",
	})
}
