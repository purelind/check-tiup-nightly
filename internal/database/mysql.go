package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/purelind/check-tiup-nightly/internal/checker"
	"github.com/purelind/check-tiup-nightly/pkg/logger"
)

type DB struct {
	db *sql.DB
}

// database configuration
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

type QueryType string

const (
	QueryByDays  QueryType = "by_days"
	QueryByLimit QueryType = "by_limit"
)

type QueryParams struct {
	Platform   string
	Days      int
	Limit     int
	QueryType QueryType
}

func New(cfg Config) (*DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)

	if !strings.Contains(dsn, "?") {
		dsn += "?"
	} else {
		dsn += "&"
	}
	dsn += "tls=preferred"

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// set connection pool parameters
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// validate database connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{db: db}, nil
}

func (db *DB) Close() error {
	return db.db.Close()
}

// initialize database table
func (db *DB) InitSchema(ctx context.Context) error {
	query := `
        CREATE TABLE IF NOT EXISTS check_results (
            id INT AUTO_INCREMENT PRIMARY KEY,
            timestamp DATETIME NOT NULL,
            status VARCHAR(50) NOT NULL,
            platform VARCHAR(50) NOT NULL,
            os VARCHAR(50) NOT NULL,
            arch VARCHAR(50) NOT NULL,
            errors JSON,
            tiup_version TEXT,
            python_version TEXT,
            components_info JSON,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            INDEX idx_platform_timestamp (platform, timestamp)
        )
    `

	if _, err := db.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("failed to create check_results table: %w", err)
	}

	return nil
}

// save check result
func (db *DB) SaveCheckResult(ctx context.Context, report *checker.CheckReport) error {
	query := `
        INSERT INTO check_results 
        (timestamp, status, platform, os, arch, errors, tiup_version, python_version, components_info)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    `

	// serialize JSON fields
	errorsJSON, err := json.Marshal(report.Errors)
	if err != nil {
		return fmt.Errorf("failed to marshal errors: %w", err)
	}

	componentsJSON, err := json.Marshal(report.Version.Components)
	if err != nil {
		return fmt.Errorf("failed to marshal components: %w", err)
	}

	_, err = db.db.ExecContext(ctx, query,
		report.Timestamp,
		report.Status,
		report.Platform,
		report.OS,
		report.Arch,
		errorsJSON,
		report.Version.TiUP,
		report.Version.Python,
		componentsJSON,
	)

	if err != nil {
		return fmt.Errorf("failed to insert check result: %w", err)
	}

	return nil
}

// GetLatestResults get the latest results of all platforms
func (db *DB) GetLatestResults(ctx context.Context) ([]checker.CheckReport, error) {
	query := `
        WITH RankedResults AS (
            SELECT *,
                ROW_NUMBER() OVER (PARTITION BY platform ORDER BY timestamp DESC) as rn
            FROM check_results
            WHERE platform IN (?, ?, ?, ?)
        )
        SELECT id, timestamp, status, platform, os, arch, 
               errors, tiup_version, components_info, created_at
        FROM RankedResults
        WHERE rn = 1
    `
	
	args := []interface{}{
		"linux-amd64", "linux-arm64",
		"darwin-amd64", "darwin-arm64",
	}
	
	return db.queryResults(ctx, query, args...)
}

// GetPlatformResults get the latest results of a specified platform
func (db *DB) GetPlatformResults(ctx context.Context, params QueryParams) ([]checker.CheckReport, error) {
	var query string
	var args []interface{}

	switch params.QueryType {
	case QueryByDays:
		query = `
            SELECT id, timestamp, status, platform, os, arch, errors, tiup_version, components_info, created_at FROM check_results
            WHERE platform = ?
            AND timestamp >= DATE_SUB(NOW(), INTERVAL ? DAY)
            ORDER BY timestamp DESC
        `
		args = []interface{}{params.Platform, params.Days}
	case QueryByLimit:
		query = `
            SELECT id, timestamp, status, platform, os, arch, errors, tiup_version, components_info, created_at FROM check_results 
            WHERE platform = ?
            ORDER BY timestamp DESC 
            LIMIT ?
        `
		args = []interface{}{params.Platform, params.Limit}
	}

	return db.queryResults(ctx, query, args...)
}

// GetPlatformHistory get the history records of a specified platform
func (db *DB) GetPlatformHistory(ctx context.Context, params QueryParams) ([]checker.CheckReport, error) {
	query := `
        SELECT * FROM check_results 
        WHERE platform = ?
        AND timestamp >= ?
        ORDER BY timestamp DESC
    `

	daysAgo := time.Now().AddDate(0, 0, -params.Days)
	return db.queryResults(ctx, query, params.Platform, daysAgo)
}

// common query results processing
func (db *DB) queryResults(ctx context.Context, query string, args ...interface{}) ([]checker.CheckReport, error) {
	rows, err := db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	var results []checker.CheckReport
	for rows.Next() {
		var report checker.CheckReport
		var errorsJSON, componentsJSON sql.NullString
		var timestamp time.Time
		var id sql.NullInt64
		var createdAt time.Time

		err := rows.Scan(
			&id,
			&timestamp,
			&report.Status,
			&report.Platform,
			&report.OS,
			&report.Arch,
			&errorsJSON,
			&report.Version.TiUP,
			&componentsJSON,
			&createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		report.Timestamp = timestamp

		// parse JSON fields
		if errorsJSON.Valid {
			if err := json.Unmarshal([]byte(errorsJSON.String), &report.Errors); err != nil {
				logger.Error("Failed to unmarshal errors JSON:", err)
			}
		}

		if componentsJSON.Valid {
			if err := json.Unmarshal([]byte(componentsJSON.String), &report.Version.Components); err != nil {
				logger.Error("Failed to unmarshal components JSON:", err)
			}
		}

		results = append(results, report)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return results, nil
}
