package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/robfig/cron/v3"
	"github.com/purelind/check-tiup-nightly/internal/checker"
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

	// setup cron job
	c := cron.New()
	_, err = c.AddFunc(cfg.CronSchedule, func() {
		updateBranchCommits(ctx, db)
	})
	if err != nil {
		logger.Error("Failed to schedule cron job:", err)
		os.Exit(1)
	} else {
		logger.Info("Cron job scheduled:", cfg.CronSchedule)
	}
	c.Start()
	defer c.Stop()

	// start server in a goroutine
	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			logger.Error("Failed to start server:", err)
			os.Exit(1)
		}
	}()

	// handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown:", err)
	}
}

func updateBranchCommits(ctx context.Context, db *database.DB) {
	components := []string{"tidb", "tikv", "pd", "tiflash"}
	for _, component := range components {
		// Fetch the latest commit info from GitHub API
		info, err := checker.FetchLatestCommitInfo(ctx, component, "master")
		if err != nil {
			logger.Error("Failed to fetch commit info for", component, ":", err)
			continue
		}

		// Update the database with the latest commit info
		if err := db.UpdateBranchCommit(ctx, info); err != nil {
			logger.Error("Failed to update commit info for", component, ":", err)
		} else {
			// log the latest commit info
			logger.Info("Updated commit info for ", component, " ", info.Branch, ": ", info.GitHash)
		}
	}
}
