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
	// 命令行参数
	// configPath := flag.String("config", "", "path to config file")
	timeout := flag.Duration("timeout", 10*time.Minute, "checker timeout duration")
	flag.Parse()

	// 加载配置
	cfg := config.Load()

	// 初始化日志
	if err := logger.Init(cfg.LogPath); err != nil {
		panic("failed to initialize logger: " + err.Error())
	}

	// 创建上下文，支持超时控制
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// 创建并运行检查器
	c := checker.New()
	success := c.Run(ctx)

	if !success {
		logger.Error("Checker failed")
		os.Exit(1)
	}

	logger.Info("Checker completed successfully")
}
