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
	flag.Parse()

	cfg := config.Load()

	// initialize logger
	if err := logger.Init(cfg.LogPath); err != nil {
		panic("failed to initialize logger: " + err.Error())
	}

	// initialize database connection
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

	// initialize database schema
	ctx := context.Background()
	if err := db.InitSchema(ctx); err != nil {
		logger.Error("Failed to initialize database schema:", err)
		os.Exit(1)
	}

	// create and start server
	srv := server.New(db, cfg.Server.Port)

	// graceful shutdown handler
	go func() {
		if err := srv.Start(); err != nil {
			logger.Error("Server error:", err)
		}
	}()

	// wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// graceful shutdown
	logger.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown:", err)
	}

	logger.Info("Server exited")
}
