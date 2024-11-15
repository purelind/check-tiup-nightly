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
	// 设置 gin 模式
	// gin.SetMode(gin.ReleaseMode)
	gin.SetMode(gin.DebugMode)

	engine := gin.New()

	// 使用 gin 的中间件
	engine.Use(gin.Recovery())
	engine.Use(RequestLogger())
	engine.Use(ErrorHandler())

	// 创建处理器
	h := NewHandler(db)

	// 注册路由
	api := engine.Group("/api/v1")
	{
		api.POST("/status", h.ReportStatus)
		api.GET("/results/latest", h.GetLatestResults)
		api.GET("/platforms/:platform/results", h.GetPlatformResults)
		api.GET("/results/platforms/:platform/history", h.GetPlatformHistory)
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

// 自定义中间件：请求日志
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// 处理请求
		c.Next()

		// 请求处理完成后记录日志
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

// 自定义中间件：错误处理
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// 检查是否有错误
		if len(c.Errors) > 0 {
			// 获取最后一个错误
			err := c.Errors.Last()

			// 根据错误类型返回适当的响应
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
