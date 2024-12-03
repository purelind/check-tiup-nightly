package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/purelind/check-tiup-nightly/internal/database"
	"github.com/purelind/check-tiup-nightly/pkg/logger"
)

type Server struct {
	engine *gin.Engine
	server *http.Server
	db     *database.DB
}

func New(db *database.DB, port int) *Server {
	gin.SetMode(gin.ReleaseMode)

	engine := gin.New()

	engine.Use(gin.Recovery())
	engine.Use(RequestLogger())
	engine.Use(ErrorHandler())

	h := NewHandler(db)

	// register routes
	api := engine.Group("/api/v1")
	{
		api.POST("/status", h.ReportStatus)
		api.GET("/results/latest", h.GetLatestResults)
		api.GET("/platforms/:platform/results", h.GetPlatformResults)
		api.GET("/results/platforms/:platform/history", h.GetPlatformHistory)
		api.POST("/branch-commits", h.UpdateBranchCommit)
		api.GET("/branch-commits", h.GetBranchCommits)
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      engine,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &Server{
		engine: engine,
		server: srv,
		db:     db,
	}
}

func (s *Server) Start() error {
	logger.Info("Starting server on", s.server.Addr)
	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	logger.Info("Shutting down server...")
	return s.server.Shutdown(ctx)
}

// custom middleware: request logger
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// process request
		c.Next()

		// log after request processed
		if raw != "" {
			path = path + "?" + raw
		}

		logger.Info(fmt.Sprintf("[GIN] %v | %3d | %13v | %15s | %-7s %s",
			time.Now().Format("2006/01/02 - 15:04:05"),
			c.Writer.Status(),
			time.Since(start),
			c.ClientIP(),
			c.Request.Method,
			path,
		))
	}
}

// custom middleware: error handler
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// check if there are errors
		if len(c.Errors) > 0 {
			// get the last error
			err := c.Errors.Last()

			// return appropriate response based on error type
			switch e := err.Err.(type) {
			case *Error:
				c.JSON(e.Status, gin.H{
					"status":  "error",
					"message": e.Message,
				})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{
					"status":  "error",
					"message": "Internal Server Error",
				})
			}
		}
	}
}
