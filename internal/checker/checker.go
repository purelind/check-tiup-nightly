package checker

import (
	"context"
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
	"os"
	"encoding/json"
	"net/http"
	"bytes"
	"syscall"

	"github.com/purelind/check-tiup-nightly/pkg/logger"
)

type Checker struct {
	platformInfo PlatformInfo
	errors       []Error
	versions     Versions
	apiEndpoint  string
}

func New() *Checker {
	return &Checker{
		platformInfo: getPlatformInfo(),
		errors:       make([]Error, 0),
		versions: Versions{
			Components: make(map[string]ComponentVersion),
		},
		apiEndpoint: getEnv("API_ENDPOINT", "http://localhost:5050/api/v1/status"),
	}
}

func getPlatformInfo() PlatformInfo {
	os := runtime.GOOS
	arch := runtime.GOARCH

	// 规范化架构名称
	if arch == "x86_64" {
		arch = "amd64"
	}

	return PlatformInfo{
		OS:       os,
		Arch:     arch,
		Platform: fmt.Sprintf("%s-%s", os, arch),
	}
}

func (c *Checker) recordError(stage, errMsg string) {
	err := Error{
		Stage:     stage,
		Error:     errMsg,
		Timestamp: time.Now(),
	}
	c.errors = append(c.errors, err)
	logger.Error(fmt.Sprintf("[%s] %s", stage, errMsg))
}

func (c *Checker) checkTiUPDownload(ctx context.Context) error {
	logger.Info("Starting TiUP download check")

	// 更新 TiUP 自身
	if err := c.runCommand(ctx, "tiup", "update", "--self"); err != nil {
		c.recordError("download", fmt.Sprintf("Failed to update TiUP: %v", err))
		return err
	}

	// 安装所需组件
	components := []string{
		"tidb:nightly",
		"tikv:nightly",
		"pd:nightly",
		"tiflash:nightly",
		"prometheus:nightly",
		"grafana:nightly",
	}

	for _, comp := range components {
		if err := c.runCommand(ctx, "tiup", "install", comp); err != nil {
			c.recordError("download", fmt.Sprintf("Failed to install %s: %v", comp, err))
			return err
		}
		logger.Info(fmt.Sprintf("Successfully installed %s", comp))
	}

	return nil
}

func (c *Checker) startPlayground(ctx context.Context) (*exec.Cmd, error) {
	logger.Info("Starting TiUP playground")

	cmd := exec.CommandContext(ctx, "tiup", "playground", "nightly")

	// 设置输出管道
	_, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	_, err = cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	// 启动进程
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start playground: %v", err)
	}

	// 等待初始化
	time.Sleep(10 * time.Second)

	// 检查数据库连接
	db, err := sql.Open("mysql", fmt.Sprintf("root@tcp(127.0.0.1:4000)/"))
	if err != nil {
		c.recordError("playground", "Failed to connect to TiDB")
		return nil, err
	}
	defer db.Close()

	// 尝试连接12次，每次间隔10秒
	for i := 0; i < 12; i++ {
		if err := db.Ping(); err == nil {
			logger.Info("Successfully connected to TiDB")
			return cmd, nil
		}

		// 检查进程是否已退出
		if cmd.ProcessState != nil {
			c.recordError("playground", "Playground process exited unexpectedly")
			return nil, fmt.Errorf("playground process exited")
		}

		time.Sleep(10 * time.Second)
	}

	return nil, fmt.Errorf("timeout waiting for TiDB to be ready")
}

func (c *Checker) runSmokeTest(ctx context.Context) error {
	logger.Info("==================== Starting smoke tests ====================")

	db, err := sql.Open("mysql", "root@tcp(127.0.0.1:4000)/")
	if err != nil {
		c.recordError("smoke_test", fmt.Sprintf("Failed to connect: %v", err))
		return err
	}
	defer db.Close()

	// 基础SQL测试
	tests := []struct {
		name string
		sql  string
	}{
		{"Create database", "CREATE DATABASE IF NOT EXISTS test"},
		{"Select database", "USE test"},
		{"Create table", "CREATE TABLE IF NOT EXISTS smoke_test (id INT PRIMARY KEY, value VARCHAR(255))"},
		{"Insert data", "INSERT INTO smoke_test VALUES (1, 'test')"},
		{"Query data", "SELECT * FROM smoke_test"},
	}

	for _, test := range tests {
		logger.Info(fmt.Sprintf("Running test: %s", test.name))
		if _, err := db.ExecContext(ctx, test.sql); err != nil {
			c.recordError("smoke_test", fmt.Sprintf("%s failed: %v", test.name, err))
			return err
		}
		logger.Info(fmt.Sprintf("✓ Passed: %s", test.name))
	}

	logger.Info("Running version consistency check...")
	if err := c.checkVersionConsistency(ctx, db); err != nil {
		logger.Error(fmt.Sprintf("Version consistency check failed: %v", err))
		return err
	}

	logger.Info("==================== Smoke tests completed successfully ====================")
	return nil
}

func (c *Checker) checkVersionConsistency(ctx context.Context, db *sql.DB) error {
	logger.Info("Checking version consistency...")
	rows, err := db.QueryContext(ctx, "SELECT * FROM information_schema.cluster_info")
	if err != nil {
		c.recordError("version_check", fmt.Sprintf("Failed to query cluster info: %v", err))
		return err
	}
	defer rows.Close()

	var referenceVersion string
	logger.Info("Scanning component versions...")
	
	for rows.Next() {
		var (
			componentType, instance, statusAddr string
			version, gitHash, startTime, uptime string
			serverId int
		)

		if err := rows.Scan(&componentType, &instance, &statusAddr, &version, &gitHash, 
			&startTime, &uptime, &serverId); err != nil {
			logger.Error(fmt.Sprintf("Failed to scan row: %v", err))
			continue
		}

		if !isValidComponent(componentType) {
			logger.Info(fmt.Sprintf("Skipping invalid component: %s", componentType))
			continue
		}

		logger.Info(fmt.Sprintf("Component: %s, Version: %s, GitHash: %s", 
			componentType, version, gitHash))

		// 验证Git hash
		if len(gitHash) != 40 {
			c.recordError("version_check", fmt.Sprintf("Invalid git hash for %s: %s", componentType, gitHash))
			return fmt.Errorf("invalid git hash")
		}

		// 提取基础版本
		baseVersion := extractBaseVersion(version)

		c.versions.Components[componentType] = ComponentVersion{
			FullVersion: version,
			BaseVersion: baseVersion,
			GitHash:     gitHash,
		}

		if referenceVersion == "" {
			referenceVersion = baseVersion
		} else if baseVersion != referenceVersion {
			c.recordError("version_check", fmt.Sprintf("Version mismatch: %s has version %s, expected %s",
				componentType, baseVersion, referenceVersion))
			return fmt.Errorf("version mismatch")
		}
	}

	logger.Info("Version consistency check completed")
	return nil
}

func (c *Checker) Run(ctx context.Context) bool {
	logger.Info("==================== Starting TiUP checker ====================")
	logger.Info(fmt.Sprintf("Platform: %s, OS: %s, Arch: %s", 
		c.platformInfo.Platform, c.platformInfo.OS, c.platformInfo.Arch))

	status := "success"
	var playground *exec.Cmd

	// 下载检查
	logger.Info("Step 1: Checking TiUP downloads...")
	if err := c.checkTiUPDownload(ctx); err != nil {
		logger.Error(fmt.Sprintf("Download check failed: %v", err))
		status = "failed"
		return false
	}
	logger.Info("Download check completed successfully")

	// 清理 tiup playground 进程
	defer func() {
		if playground != nil && playground.Process != nil {
			logger.Info("Cleaning up: Gracefully stopping playground process")
			// 首先发送 SIGTERM
			if err := playground.Process.Signal(syscall.SIGTERM); err != nil {
				logger.Error(fmt.Sprintf("Failed to send SIGTERM: %v", err))
			}

			// 给予进程最多10秒的时间来清理
			done := make(chan error, 1)
			go func() {
				done <- playground.Wait()
			}()

			select {
			case <-time.After(10 * time.Second):
				logger.Info("Process didn't exit in time, forcing kill")
				if err := playground.Process.Kill(); err != nil {
					logger.Error(fmt.Sprintf("Failed to kill playground: %v", err))
				}
			case err := <-done:
				if err != nil {
					logger.Error(fmt.Sprintf("Process exited with error: %v", err))
				} else {
					logger.Info("Process exited gracefully")
				}
			}
		}
	}()

	logger.Info("Step 2: Starting playground...")
	var err error
	playground, err = c.startPlayground(ctx)
	if err != nil {
		logger.Error(fmt.Sprintf("Playground startup failed: %v", err))
		status = "failed"
	} else {
		logger.Info("Step 3: Running smoke tests...")
		if err := c.runSmokeTest(ctx); err != nil {
			logger.Error(fmt.Sprintf("Smoke tests failed: %v", err))
			status = "failed"
		} else {
			logger.Info("Smoke tests completed successfully")
		}
	}

	// Get TiUP version before sending report
	c.versions.TiUP = c.getTiUPVersion()

	// Send report
	logger.Info("Step 4: Sending report...")
	if err := c.sendReport(ctx, status); err != nil {
		logger.Error(fmt.Sprintf("Failed to send report: %v", err))
	} else {
		logger.Info("Report sent successfully")
	}

	logger.Info(fmt.Sprintf("==================== Check completed with status: %s ====================", status))
	return status == "success"
}

// 辅助函数
func isValidComponent(component string) bool {
	validComponents := map[string]bool{
		"tidb":    true,
		"pd":      true,
		"tikv":    true,
		"tiflash": true,
	}
	return validComponents[component]
}

func extractBaseVersion(version string) string {
	parts := strings.Split(version, "-")
	if len(parts) >= 2 {
		return strings.Join(parts[0:2], "-")
	}
	return version
}

func (c *Checker) runCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %v, output: %s", err, string(output))
	}
	return nil
}

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}

func (c *Checker) sendReport(ctx context.Context, status string) error {
	report := CheckReport{
		Timestamp:  time.Now(),
		Status:     status,
		Platform:   c.platformInfo.Platform,
		OS:         c.platformInfo.OS,
		Arch:       c.platformInfo.Arch,
		Errors:     c.errors,
		Version: Versions{
			TiUP:       c.versions.TiUP,
			Components: c.versions.Components,
		},
	}

	jsonData, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to marshal report: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.apiEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send report: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func (c *Checker) getTiUPVersion() string {
	cmd := exec.Command("tiup", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to get TiUP version: %v", err))
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}