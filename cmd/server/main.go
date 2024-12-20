package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/purelind/check-tiup-nightly/internal/config"
	"github.com/purelind/check-tiup-nightly/internal/database"
	"github.com/purelind/check-tiup-nightly/internal/server"
	"github.com/purelind/check-tiup-nightly/internal/updater"
	"github.com/purelind/check-tiup-nightly/pkg/logger"
	"github.com/robfig/cron/v3"
)

type App struct {
	cfg    *config.Config
	db     *database.DB
	server *server.Server
	cron   *cron.Cron
	updater *service.Updater
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

	updater := service.NewUpdater(db)

	app := &App{
		cfg:    cfg,
		db:     db,
		server: srv,
		updater: updater,
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
		if err := a.updater.UpdateAllComponentsCommits(ctx); err != nil {
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
