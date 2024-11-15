package main

import (
	"context"
	"flag"
	"os"
	"time"

	"github.com/purelind/check-tiup-nightly/internal/checker"
	"github.com/purelind/check-tiup-nightly/internal/config"
	"github.com/purelind/check-tiup-nightly/pkg/logger"
)

func main() {
	timeout := flag.Duration("timeout", 10*time.Minute, "checker timeout duration")
	flag.Parse()

	cfg := config.Load()

	if err := logger.Init(cfg.LogPath); err != nil {
		panic("failed to initialize logger: " + err.Error())
	}
	// create context with timeout control
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// create and run checker
	c := checker.NewChecker(cfg)
	success := c.Run(ctx)

	if !success {
		logger.Error("Checker failed")
		os.Exit(1)
	}

	logger.Info("Checker completed successfully")
}
