package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"tg_bot/internal/httpserver"
)

// RegisterAuthRoutes returns a route registration function for the auth handler.
func RegisterAuthRoutes(auth *Auth) httpserver.RouteFunc {
	return func(g *httpserver.HandlerGroup) {
		g.Router.GET("/api/auth/telegram/oidc/authorize", auth.OIDCAuthorize)
		g.Router.GET("/api/auth/telegram/oidc/callback", auth.OIDCCallback)
		g.Router.POST("/api/auth/logout", auth.Logout)
		g.Router.GET("/api/auth/me", auth.Me)
		g.Router.GET("/api/auth/config", auth.AuthConfigJSON)

		protected := g.Router.Group("/api/dashboard")
		protected.Use(auth.RequireAuth())
		protected.GET("/profile", func(c *gin.Context) {
			session := SessionFromContext(c)
			c.JSON(http.StatusOK, session)
		})
		protected.GET("/overview", auth.DashboardOverview)
		protected.GET("/activity", auth.ListActivity)
		protected.GET("/commands", auth.ListCommands)
		protected.GET("/built-in-commands", auth.ListBuiltInCommands)
		protected.PUT("/built-in-commands/:command", auth.UpdateBuiltInCommand)
		protected.PATCH("/built-in-commands/:command/enabled", auth.SetBuiltInCommandEnabled)
		protected.POST("/commands", auth.CreateCommand)
		protected.PUT("/commands/:id", auth.UpdateCommand)
		protected.PATCH("/commands/:id/enabled", auth.SetCommandEnabled)
		protected.DELETE("/commands/:id", auth.DeleteCommand)
	}
}
