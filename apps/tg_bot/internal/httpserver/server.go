// Package httpserver provides the Gin-based HTTP server for the bot.
package httpserver

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Server wraps a Gin engine and an http.Server.
type Server struct {
	httpServer *http.Server
	log        *slog.Logger
}

// HandlerGroup lets callers register routes on the router.
type HandlerGroup struct {
	Router *gin.Engine
}

// RouteFunc is used to register application routes.
type RouteFunc func(g *HandlerGroup)

// New builds the Gin router, registers the webhook route and a health check.
func New(addr, webhookPath string, webhookHandler gin.HandlerFunc, routeFns []RouteFunc, allowedOrigins []string, log *slog.Logger) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	if len(allowedOrigins) > 0 {
		router.Use(corsMiddleware(allowedOrigins))
	}

	router.POST(webhookPath, webhookHandler)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	group := &HandlerGroup{Router: router}
	for _, fn := range routeFns {
		fn(group)
	}

	return &Server{
		httpServer: &http.Server{
			Addr:    addr,
			Handler: router,
		},
		log: log,
	}
}

// corsMiddleware returns a Gin middleware that sets CORS headers for allowed origins.
func corsMiddleware(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		allowed := false
		for _, o := range allowedOrigins {
			if o == origin {
				allowed = true
				break
			}
			// Allow wildcard subdomains like https://*.example.com.
			if len(o) > 1 && o[0] == '*' && o[1] == '.' && len(origin) > len(o)-1 {
				suffix := o[1:]
				if len(origin) > len(suffix) && origin[len(origin)-len(suffix):] == suffix {
					allowed = true
					break
				}
			}
		}

		if allowed && origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// Start begins serving requests. It returns http.ErrServerClosed on graceful shutdown.
func (s *Server) Start() error {
	s.log.Info("starting http server", "addr", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
