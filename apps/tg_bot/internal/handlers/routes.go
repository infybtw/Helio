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

		// Example protected route that can be expanded for dashboard API.
		protected := g.Router.Group("/api/dashboard")
		protected.Use(auth.RequireAuth())
		protected.GET("/profile", func(c *gin.Context) {
			session := SessionFromContext(c)
			c.JSON(http.StatusOK, session)
		})
	}
}
