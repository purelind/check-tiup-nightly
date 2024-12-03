package checker

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/purelind/check-tiup-nightly/internal/config"
	"github.com/purelind/check-tiup-nightly/internal/notify"
	"github.com/purelind/check-tiup-nightly/pkg/logger"
)

type Checker struct {
	platformInfo PlatformInfo
	errors       []Error
	versions     Versions
	apiEndpoint  string
	githubToken  string
	notifier     *notify.Notifier
}

func NewChecker(cfg *config.Config) *Checker {
	return &Checker{
		platformInfo: getPlatformInfo(),
		errors:       make([]Error, 0),
		versions: Versions{
			Components: make(map[string]ComponentVersion),
		},
		apiEndpoint: cfg.APIEndpoint,
		githubToken: cfg.GitHubToken,
		notifier:    notify.NewNotifier(),
	}
}

func getPlatformInfo() PlatformInfo {
	os := runtime.GOOS
	arch := runtime.GOARCH

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

	if err := c.runCommand(ctx, "tiup", "update", "--self"); err != nil {
		c.recordError("download", fmt.Sprintf("Failed to update TiUP: %v", err))
		return err
	}

	// check nightly components
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

	cmd := exec.CommandContext(ctx, "tiup", "playground", "nightly",
		"--db", "1",
		"--kv", "1",
		"--pd", "1",
		"--tiflash", "1")

	_, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	_, err = cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start playground: %v", err)
	}

	// wait for initialization
	time.Sleep(10 * time.Second)

	// check database connection
	db, err := sql.Open("mysql", fmt.Sprintf("root@tcp(127.0.0.1:4000)/"))
	if err != nil {
		c.recordError("playground", "Failed to connect to TiDB")
		return nil, err
	}
	defer db.Close()

	// try to connect to TiDB 12 times, with 10 seconds interval
	for i := 0; i < 12; i++ {
		if err := db.Ping(); err == nil {
			logger.Info("Successfully connected to TiDB")
			break
		}

		// check if the process has exited
		if cmd.ProcessState != nil {
			c.recordError("playground", "Playground process exited unexpectedly")
			return nil, fmt.Errorf("playground process exited")
		}

		time.Sleep(10 * time.Second)
	}

	// Add TiFlash readiness check
	logger.Info("Waiting for TiFlash to be ready...")
	for i := 0; i < 12; i++ {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:3930", 5*time.Second)
		if err == nil {
			conn.Close()
			logger.Info("Successfully connected to TiFlash")
			return cmd, nil
		}

		// check if the process has exited
		if cmd.ProcessState != nil {
			c.recordError("playground", "Playground process exited unexpectedly while waiting for TiFlash")
			return nil, fmt.Errorf("playground process exited")
		}

		logger.Info("TiFlash not ready yet, waiting...")
		time.Sleep(5 * time.Second)
	}

	c.recordError("playground", "Timeout waiting for TiFlash to be ready")
	return nil, fmt.Errorf("timeout waiting for TiFlash to be ready")
}

func (c *Checker) runSmokeTest(ctx context.Context) error {
	logger.Info("==================== Starting smoke tests ====================")

	db, err := sql.Open("mysql", "root@tcp(127.0.0.1:4000)/")
	if err != nil {
		c.recordError("smoke_test", fmt.Sprintf("Failed to connect: %v", err))
		return err
	}
	defer db.Close()

	// basic SQL tests
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
	logger.Info("Querying information_schema.cluster_info...")
	rows, err := db.QueryContext(ctx, "SELECT * FROM information_schema.cluster_info")
	if err != nil {
		c.recordError("version_check", fmt.Sprintf("Failed to query cluster_info: %v", err))
		return err
	}
	defer rows.Close()

	var referenceVersion string
	var componentCount int
	logger.Info("Scanning component versions...")

	for rows.Next() {
		componentCount++
		var (
			componentType, instance, statusAddr string
			version, gitHash, startTime, uptime string
			serverId                            int
		)

		if err := rows.Scan(&componentType, &instance, &statusAddr, &version, &gitHash,
			&startTime, &uptime, &serverId); err != nil {
			logger.Error(fmt.Sprintf("Failed to scan row: %v", err))
			continue
		}

		logger.Info(fmt.Sprintf("Found component in cluster_info - Type: %s, Instance: %s, Status: %s",
			componentType, instance, statusAddr))

		if !isValidComponent(componentType) {
			logger.Info(fmt.Sprintf("Skipping invalid component: %s (not in allowed list: tidb, pd, tikv, tiflash)", componentType))
			continue
		}

		commitTime, err := c.getGitHubCommitTime(ctx, componentType, gitHash)
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to get commit time for %s: %v", componentType, err))
			commitTime = time.Time{} // if failed, use zero value
		}

		logger.Info(fmt.Sprintf("Component: %s, Version: %s, GitHash: %s, CommitTime: %s",
			componentType, version, gitHash, commitTime))

		// validate git hash
		if len(gitHash) != 40 {
			c.recordError("version_check", fmt.Sprintf("Invalid git hash for %s: %s", componentType, gitHash))
			return fmt.Errorf("invalid git hash")
		}

		// extract base version
		baseVersion := extractBaseVersion(version)

		c.versions.Components[componentType] = ComponentVersion{
			FullVersion: version,
			BaseVersion: baseVersion,
			GitHash:     gitHash,
			CommitTime:  commitTime,
		}

		if referenceVersion == "" {
			referenceVersion = baseVersion
		} else if baseVersion != referenceVersion {
			c.recordError("version_check", fmt.Sprintf("Version mismatch: %s has version %s, expected %s",
				componentType, baseVersion, referenceVersion))
			return fmt.Errorf("version mismatch")
		}
	}

	logger.Info(fmt.Sprintf("Version consistency check completed. Total components found: %d", componentCount))
	logger.Info(fmt.Sprintf("Components registered: %v", getMapKeys(c.versions.Components)))
	return nil
}

func (c *Checker) Run(ctx context.Context) bool {
	logger.Info("==================== Starting TiUP checker ====================")
	logger.Info(fmt.Sprintf("Platform: %s, OS: %s, Arch: %s",
		c.platformInfo.Platform, c.platformInfo.OS, c.platformInfo.Arch))

	status := "success"
	var playground *exec.Cmd

	// download components check
	logger.Info("Step 1: Checking TiUP downloads...")
	if err := c.checkTiUPDownload(ctx); err != nil {
		logger.Error(fmt.Sprintf("Download check failed: %v", err))
		status = "failed"
		return false
	}
	logger.Info("Download check completed successfully")

	// clean up tiup playground process
	defer func() {
		if playground != nil && playground.Process != nil {
			logger.Info("Cleaning up: Gracefully stopping playground process")
			// first send SIGTERM
			if err := playground.Process.Signal(syscall.SIGTERM); err != nil {
				logger.Error(fmt.Sprintf("Failed to send SIGTERM: %v", err))
			}

			// give the process up to 10 seconds to clean up
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

	// send notification after sending report
	if len(c.errors) > 0 {
		errors := make([]notify.ErrorDetail, 0, len(c.errors))
		for _, err := range c.errors {
			errors = append(errors, notify.ErrorDetail{
				Stage:     err.Stage,
				Error:     err.Error,
				Timestamp: err.Timestamp,
			})
		}
		if err := c.notifier.SendFailureNotification(c.platformInfo.Platform, c.versions.TiUP, errors); err != nil {
			logger.Error(fmt.Sprintf("Failed to send failure notification: %v", err))
		}
		return false
	} else {
		if err := c.notifier.SendSuccessNotification(c.platformInfo.Platform, c.versions.TiUP); err != nil {
			logger.Error(fmt.Sprintf("Failed to send success notification: %v", err))
		}
		return true
	}
}

// helper functions
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

func (c *Checker) sendReport(ctx context.Context, status string) error {
	report := CheckReport{
		Timestamp: time.Now().UTC(),
		Status:    status,
		Platform:  c.platformInfo.Platform,
		OS:        c.platformInfo.OS,
		Arch:      c.platformInfo.Arch,
		Errors:    c.errors,
		Version: Versions{
			TiUP:       c.versions.TiUP,
			Components: c.versions.Components,
		},
	}

	jsonData, err := json.Marshal(report)
	logger.Info(fmt.Sprintf("Sending report: %s", string(jsonData)))
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

func (c *Checker) getGitHubCommitTime(ctx context.Context, component, hash string) (time.Time, error) {
	// component to repository mapping
	repoMap := map[string]string{
		"tidb":    "pingcap/tidb",
		"tikv":    "tikv/tikv",
		"pd":      "tikv/pd",
		"tiflash": "pingcap/tiflash",
	}

	repo, ok := repoMap[component]
	if !ok {
		return time.Time{}, fmt.Errorf("unknown component: %s", component)
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/commits/%s", repo, hash)
	logger.Info(fmt.Sprintf("Requesting GitHub API: %s", url))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return time.Time{}, err
	}

	if c.githubToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.githubToken))
	} else {
		logger.Error("GitHub token is empty")
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Error(fmt.Sprintf("GitHub API error - Status: %d, Response: %s", resp.StatusCode, string(body)))
		return time.Time{}, fmt.Errorf("GitHub API returned status: %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Commit struct {
			Committer struct {
				Date time.Time `json:"date"`
			} `json:"committer"`
		} `json:"commit"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return time.Time{}, err
	}

	return result.Commit.Committer.Date, nil
}

func getMapKeys(m map[string]ComponentVersion) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
