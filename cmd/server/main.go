package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/purelind/check-tiup-nightly/internal/config"
	"github.com/purelind/check-tiup-nightly/internal/database"
	"github.com/purelind/check-tiup-nightly/internal/server"
	"github.com/purelind/check-tiup-nightly/pkg/logger"
)

func main() {
	// 命令行参数
	// configPath := flag.String("config", "", "path to config file")
	flag.Parse()

	// 加载配置
	cfg := config.Load()

	// 初始化日志
	if err := logger.Init(cfg.LogPath); err != nil {
		panic("failed to initialize logger: " + err.Error())
	}

	// 初始化数据库连接
	db, err := database.New(database.Config{
		Host:     cfg.MySQL.Host,
		Port:     cfg.MySQL.Port,
		User:     cfg.MySQL.User,
		Password: cfg.MySQL.Password,
		Database: cfg.MySQL.Database,
	})
	if err != nil {
		logger.Error("Failed to connect to database:", err)
		os.Exit(1)
	}
	defer db.Close()

	// 初始化数据库schema
	ctx := context.Background()
	if err := db.InitSchema(ctx); err != nil {
		logger.Error("Failed to initialize database schema:", err)
		os.Exit(1)
	}

	// 创建并启动服务器
	srv := server.New(db, cfg.Server.Port)

	// 优雅关闭处理
	go func() {
		if err := srv.Start(); err != nil {
			logger.Error("Server error:", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// 优雅关闭
	logger.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown:", err)
	}

	logger.Info("Server exited")
}
