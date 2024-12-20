package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/purelind/check-tiup-nightly/internal/checker"
	"github.com/purelind/check-tiup-nightly/internal/config"
	"github.com/purelind/check-tiup-nightly/internal/database"
	"github.com/purelind/check-tiup-nightly/internal/server"
	"github.com/purelind/check-tiup-nightly/pkg/logger"
	"github.com/robfig/cron/v3"
)

type App struct {
	cfg    *config.Config
	db     *database.DB
	server *server.Server
	cron   *cron.Cron
}

func main() {
	flag.Parse()

	app, err := initApp()
	if err != nil {
		logger.Error("Failed to initialize application:", err)
		os.Exit(1)
	}
	defer app.cleanup()

	if err := app.run(); err != nil {
		logger.Error("Application error:", err)
		os.Exit(1)
	}
}

func initApp() (*App, error) {
	cfg := config.Load()

	if err := logger.Init(cfg.LogPath); err != nil {
		return nil, err
	}

	db, err := initDatabase(cfg)
	if err != nil {
		return nil, err
	}

	srv := server.New(db, cfg.Server.Port)

	app := &App{
		cfg:    cfg,
		db:     db,
		server: srv,
	}

	if cfg.EnableCron {
		if err := app.initCronJob(); err != nil {
			return nil, err
		}
	} else {
		logger.Info("Cron job is disabled")
	}

	return app, nil
}

func initDatabase(cfg *config.Config) (*database.DB, error) {
	db, err := database.New(database.Config{
		Host: cfg.MySQL.Host,
		Port: cfg.MySQL.Port,
		User: cfg.MySQL.User,

		Password: cfg.MySQL.Password,
		Database: cfg.MySQL.Database,
	})
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	if err := db.InitSchema(ctx); err != nil {
		return nil, err
	}

	return db, nil
}

func (a *App) initCronJob() error {
	a.cron = cron.New()
	_, err := a.cron.AddFunc(a.cfg.CronSchedule, func() {
		ctx := context.Background()
		if err := a.updateAllComponentsCommits(ctx); err != nil {
			logger.Error("Failed to update components commits:", err)
		}
	})
	if err != nil {
		return err
	}

	logger.Info("Cron job scheduled:", a.cfg.CronSchedule)
	a.cron.Start()
	return nil
}

func (a *App) run() error {
	// Start server in a goroutine
	go func() {
		if err := a.server.Start(); err != nil && err != http.ErrServerClosed {
			logger.Error("Failed to start server:", err)
			os.Exit(1)
		}
	}()

	return a.waitForShutdown()
}

func (a *App) waitForShutdown() error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return a.server.Shutdown(ctx)
}

func (a *App) cleanup() {
	if a.db != nil {
		a.db.Close()
	}
	if a.cron != nil {
		a.cron.Stop()
	}
}

func (a *App) updateAllComponentsCommits(ctx context.Context) error {
	components := []string{"tidb", "tikv", "pd", "tiflash"}

	for _, component := range components {
		if err := a.updateComponentCommit(ctx, component); err != nil {
			logger.Error("Failed to update commit info for", component, ":", err)
			continue
		}
	}
	return nil
}

func (a *App) updateComponentCommit(ctx context.Context, component string) error {
	info, err := checker.FetchLatestCommitInfo(ctx, component, "master")
	if err != nil {
		return err
	}

	if err := a.db.UpdateBranchCommit(ctx, info); err != nil {
		return err
	}

	logger.Info("Updated commit info for", component, info.Branch, ":", info.GitHash)
	return nil
}
