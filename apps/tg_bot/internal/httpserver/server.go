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

// New builds the Gin router, registers the webhook route and a health check.
func New(addr, webhookPath string, webhookHandler gin.HandlerFunc, log *slog.Logger) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	router.POST(webhookPath, webhookHandler)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	return &Server{
		httpServer: &http.Server{
			Addr:    addr,
			Handler: router,
		},
		log: log,
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
