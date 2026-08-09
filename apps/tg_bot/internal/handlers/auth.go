package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"tg_bot/internal/auth"
	"tg_bot/internal/commands"
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
	ownedChatIDs, err := a.ownedChatIDs(c)
	if err != nil {
		a.log.Error("failed to list dashboard chats", "error", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to load dashboard data"})
		return
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

func (a *Auth) ListActivity(c *gin.Context) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "page must be a positive integer"})
		return
	}
	eventType := c.Query("type")
	if eventType != "" && eventType != "custom" && eventType != "moderation" && eventType != "info" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid activity type"})
		return
	}
	chatIDs, err := a.ownedChatIDs(c)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to load activity"})
		return
	}
	activity, err := a.db.ListActivity(c.Request.Context(), chatIDs, eventType, 50, (page-1)*50)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to load activity"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": activity.Items, "total": activity.Total, "page": page, "per_page": 50})
}

func (a *Auth) ownedChatIDs(c *gin.Context) ([]int64, error) {
	chatIDs, err := a.db.ListTrackedChatIDs(c.Request.Context())
	if err != nil {
		return nil, err
	}
	session := SessionFromContext(c)
	owned := make([]int64, 0, len(chatIDs))
	for _, chatID := range chatIDs {
		member, err := a.client.GetChatMember(c.Request.Context(), chatID, session.UserID)
		if err == nil && member.Status == "creator" {
			owned = append(owned, chatID)
		}
	}
	if selected := c.Query("chat_id"); selected != "" {
		selectedID, err := strconv.ParseInt(selected, 10, 64)
		a.log.Info("dashboard chat scope requested", "requested_chat_id", selected, "parsed_chat_id", selectedID, "owned_chat_ids", owned)
		if err != nil || selectedID == 0 || !containsChatID(owned, selectedID) {
			return []int64{}, nil
		}
		a.log.Info("dashboard chat scope accepted", "chat_id", selectedID)
		return []int64{selectedID}, nil
	}
	return owned, nil
}

func (a *Auth) ListCommands(c *gin.Context) {
	chatIDs, err := a.ownedChatIDs(c)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to load commands"})
		return
	}
	commands, err := a.db.ListCustomCommands(c.Request.Context(), chatIDs)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to load commands"})
		return
	}
	a.log.Info("dashboard commands loaded", "requested_chat_id", c.Query("chat_id"), "chat_ids", chatIDs, "command_count", len(commands))
	c.JSON(http.StatusOK, gin.H{"commands": commands})
}

type builtInCommandResponse struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Permission   string `json:"permission"`
	Enabled      bool   `json:"enabled"`
	MuteDuration string `json:"mute_duration"`
	ReplyMessage string `json:"reply_message"`
}

func (a *Auth) ListBuiltInCommands(c *gin.Context) {
	chatID, ok := a.ownedSelectedChatID(c)
	if !ok {
		return
	}
	settings, err := a.db.ListBuiltInCommandSettings(c.Request.Context(), chatID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to load built-in command settings"})
		return
	}
	settingsByCommand := make(map[string]database.BuiltInCommandSetting, len(settings))
	for _, setting := range settings {
		settingsByCommand[setting.Command] = setting
	}
	response := make([]builtInCommandResponse, 0)
	for _, command := range commands.BuiltInCommands() {
		responseCommand := builtInCommandResponse{Name: command.Name, Description: command.Description, Permission: command.Permission, Enabled: true}
		if command.Name == "!mute" {
			responseCommand.MuteDuration = "30m"
		}
		if command.Name == "!help" {
			responseCommand.ReplyMessage = commands.HelpText()
		}
		if setting, exists := settingsByCommand[command.Name]; exists {
			responseCommand.Enabled = setting.Enabled
			if setting.Permission != "" {
				responseCommand.Permission = setting.Permission
			}
			if setting.MuteDuration != "" {
				responseCommand.MuteDuration = setting.MuteDuration
			}
			if setting.ReplyMessage != "" || command.Name != "!help" {
				responseCommand.ReplyMessage = setting.ReplyMessage
			}
		}
		response = append(response, responseCommand)
	}
	c.JSON(http.StatusOK, gin.H{"commands": response})
}

type updateBuiltInCommandRequest struct {
	Enabled      bool   `json:"enabled"`
	Permission   string `json:"permission"`
	MuteDuration string `json:"mute_duration"`
	ReplyMessage string `json:"reply_message"`
}

func (a *Auth) UpdateBuiltInCommand(c *gin.Context) {
	chatID, ok := a.ownedSelectedChatID(c)
	if !ok {
		return
	}
	commandName := strings.ToLower(strings.TrimSpace(c.Param("command")))
	if !isBuiltInCommand(commandName) {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "built-in command not found"})
		return
	}
	var request updateBuiltInCommandRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	request.Permission = strings.ToLower(strings.TrimSpace(request.Permission))
	request.MuteDuration = strings.TrimSpace(request.MuteDuration)
	request.ReplyMessage = strings.TrimSpace(request.ReplyMessage)
	if request.Permission != "user" && request.Permission != "moderator" && request.Permission != "owner" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid command permission"})
		return
	}
	if commandName == "!mute" {
		if request.MuteDuration == "" {
			request.MuteDuration = "30m"
		}
		if _, err := commands.ParseDuration(request.MuteDuration); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid mute duration"})
			return
		}
	} else if request.MuteDuration != "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "mute duration is only available for !mute"})
		return
	}
	if len(request.ReplyMessage) > 4096 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "reply message is too long"})
		return
	}
	setting := database.BuiltInCommandSetting{
		ChatID: chatID, Command: commandName, Enabled: request.Enabled, Permission: request.Permission,
		MuteDuration: request.MuteDuration, ReplyMessage: request.ReplyMessage,
	}
	if err := a.db.UpdateBuiltInCommandSetting(c.Request.Context(), setting); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to update built-in command setting"})
		return
	}
	session := SessionFromContext(c)
	if err := a.db.RecordAction(c.Request.Context(), database.ActionRecord{
		ChatID: chatID, ActorID: session.UserID, ActorFirstName: session.FirstName,
		Action: "built-in command settings updated: " + commandName, EventType: "info",
	}); err != nil {
		a.log.Warn("failed to record built-in command setting activity", "error", err, "chat_id", chatID, "command", commandName)
	}
	if commandName == "!help" && setting.ReplyMessage == "" {
		setting.ReplyMessage = commands.HelpText()
	}
	c.JSON(http.StatusOK, setting)
}

func (a *Auth) SetBuiltInCommandEnabled(c *gin.Context) {
	chatID, ok := a.ownedSelectedChatID(c)
	if !ok {
		return
	}
	commandName := strings.ToLower(strings.TrimSpace(c.Param("command")))
	if !isBuiltInCommand(commandName) {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "built-in command not found"})
		return
	}
	var request struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := a.db.SetBuiltInCommandEnabled(c.Request.Context(), chatID, commandName, request.Enabled); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to update built-in command setting"})
		return
	}
	session := SessionFromContext(c)
	if err := a.db.RecordAction(c.Request.Context(), database.ActionRecord{
		ChatID: chatID, ActorID: session.UserID, ActorFirstName: session.FirstName,
		Action: "built-in command " + commandName + " " + map[bool]string{true: "enabled", false: "disabled"}[request.Enabled], EventType: "info",
	}); err != nil {
		a.log.Warn("failed to record built-in command setting activity", "error", err, "chat_id", chatID, "command", commandName)
	}
	c.JSON(http.StatusOK, gin.H{"command": commandName, "enabled": request.Enabled})
}

func (a *Auth) ownedSelectedChatID(c *gin.Context) (int64, bool) {
	chatID, err := strconv.ParseInt(c.Query("chat_id"), 10, 64)
	if err != nil || chatID == 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "chat_id is required"})
		return 0, false
	}
	chatIDs, err := a.ownedChatIDs(c)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to verify chat ownership"})
		return 0, false
	}
	if !containsChatID(chatIDs, chatID) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "you do not own this chat"})
		return 0, false
	}
	return chatID, true
}

func isBuiltInCommand(name string) bool {
	for _, command := range commands.BuiltInCommands() {
		if command.Name == name {
			return true
		}
	}
	return false
}

type createCommandRequest struct {
	ChatID     int64                          `json:"chat_id"`
	Name       string                         `json:"name"`
	Permission string                         `json:"permission"`
	Aliases    []string                       `json:"aliases"`
	Actions    []database.CustomCommandAction `json:"actions"`
}

func (a *Auth) CreateCommand(c *gin.Context) {
	var request createCommandRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	request.Name = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(request.Name, "!")))
	request.Aliases = normalizeAliases(request.Aliases)
	request.Permission = strings.ToLower(strings.TrimSpace(request.Permission))
	if !validCommandRequest(request) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "name and at least one message action are required"})
		return
	}
	ownedChatIDs, err := a.ownedChatIDs(c)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to verify chat ownership"})
		return
	}
	if !containsChatID(ownedChatIDs, request.ChatID) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "you do not own this chat"})
		return
	}
	session := SessionFromContext(c)
	command, err := a.db.CreateCustomCommand(c.Request.Context(), request.ChatID, session.UserID, "!"+request.Name, request.Permission, request.Aliases, request.Actions)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": "command already exists in this chat"})
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to create command"})
		return
	}
	a.recordCommandActivity(c, command, "created")
	c.JSON(http.StatusCreated, command)
}

func (a *Auth) UpdateCommand(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid command id"})
		return
	}
	var request createCommandRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	request.Name = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(request.Name, "!")))
	request.Aliases = normalizeAliases(request.Aliases)
	request.Permission = strings.ToLower(strings.TrimSpace(request.Permission))
	if !validCommandRequest(request) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "name and at least one message action are required"})
		return
	}
	chatIDs, err := a.ownedChatIDs(c)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to verify chat ownership"})
		return
	}
	if !containsChatID(chatIDs, request.ChatID) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "you do not own this chat"})
		return
	}
	command, updated, err := a.db.UpdateCustomCommand(c.Request.Context(), id, request.ChatID, chatIDs, "!"+request.Name, request.Permission, request.Aliases, request.Actions)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": "command already exists in this chat"})
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to update command"})
		return
	}
	if !updated {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "command not found"})
		return
	}
	a.recordCommandActivity(c, command, "updated")
	c.JSON(http.StatusOK, command)
}

func validCommandRequest(request createCommandRequest) bool {
	if request.ChatID == 0 || request.Name == "" || len(request.Name) > 32 || len(request.Actions) == 0 || len(request.Aliases) > 10 || (request.Permission != "user" && request.Permission != "moderator" && request.Permission != "owner") || strings.ContainsAny(request.Name, " !\t\r\n") {
		return false
	}
	seenAliases := make(map[string]struct{}, len(request.Aliases))
	for _, alias := range request.Aliases {
		if alias == "!"+request.Name || alias == "" || len(alias) > 33 || strings.ContainsAny(strings.TrimPrefix(alias, "!"), " !\t\r\n") {
			return false
		}
		if _, exists := seenAliases[alias]; exists {
			return false
		}
		seenAliases[alias] = struct{}{}
	}
	for i := range request.Actions {
		request.Actions[i].Payload = strings.TrimSpace(request.Actions[i].Payload)
		if request.Actions[i].Type != "send_message" && request.Actions[i].Type != "reply_message" && request.Actions[i].Type != "mute" && request.Actions[i].Type != "delete_message" {
			return false
		}
		if (request.Actions[i].Type == "send_message" || request.Actions[i].Type == "reply_message") && (request.Actions[i].Payload == "" || len(request.Actions[i].Payload) > 4096) {
			return false
		}
		if request.Actions[i].Type == "mute" && request.Actions[i].Payload == "" {
			request.Actions[i].Payload = "30m"
		}
		if request.Actions[i].Type == "delete_message" {
			request.Actions[i].Payload = ""
		}
		if len(request.Actions[i].Payload) > 4096 {
			return false
		}
	}
	return true
}

func normalizeAliases(aliases []string) []string {
	result := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		alias = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(alias, "!")))
		if alias != "" {
			result = append(result, "!"+alias)
		}
	}
	return result
}

func (a *Auth) DeleteCommand(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid command id"})
		return
	}
	chatIDs, err := a.ownedChatIDs(c)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to verify chat ownership"})
		return
	}
	command, deleted, err := a.db.DeleteCustomCommand(c.Request.Context(), id, chatIDs)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to delete command"})
		return
	}
	if !deleted {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "command not found"})
		return
	}
	a.recordCommandActivity(c, command, "deleted")
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *Auth) SetCommandEnabled(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid command id"})
		return
	}
	var request struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	chatIDs, err := a.ownedChatIDs(c)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to verify chat ownership"})
		return
	}
	updated, err := a.db.SetCustomCommandEnabled(c.Request.Context(), id, request.Enabled, chatIDs)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to update command status"})
		return
	}
	if !updated {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "command not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "enabled": request.Enabled})
}

func (a *Auth) recordCommandActivity(c *gin.Context, command database.CustomCommand, event string) {
	session := SessionFromContext(c)
	if err := a.db.RecordAction(c.Request.Context(), database.ActionRecord{
		ChatID:         command.ChatID,
		ActorID:        session.UserID,
		ActorFirstName: session.FirstName,
		Action:         "custom command " + event + ": " + command.Name,
		EventType:      "info",
	}); err != nil {
		a.log.Warn("failed to record custom command management activity", "error", err, "command_id", command.ID, "event", event)
	}
}

func containsChatID(chatIDs []int64, wanted int64) bool {
	for _, chatID := range chatIDs {
		if chatID == wanted {
			return true
		}
	}
	return false
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
