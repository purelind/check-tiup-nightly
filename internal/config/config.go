package config

import (
    "os"
    "strconv"
)

type Config struct {
    MySQL struct {
        Host     string
        User     string
        Password string
        Port     int
        Database string
    }
    Server struct {
        Port int
    }
    APIEndpoint string
    LogPath string
    GitHubToken string
    CronSchedule string
    EnableCron   bool
}

func Load() *Config {
    cfg := &Config{}
    
    // MySQL configuration
    cfg.MySQL.Host = getEnv("MYSQL_HOST", "localhost")
    cfg.MySQL.User = getEnv("MYSQL_USER", "root")
    cfg.MySQL.Password = getEnv("MYSQL_PASSWORD", "")
    cfg.MySQL.Port = getEnvInt("MYSQL_PORT", 4000)
    cfg.MySQL.Database = getEnv("MYSQL_DATABASE", "tiup_checks")
    
    // server configuration
    cfg.Server.Port = getEnvInt("SERVER_PORT", 5050)

    // API configuration
    cfg.APIEndpoint = getEnv("API_ENDPOINT", "http://localhost:5050/api/v1/status")
    
    // log configuration
    cfg.LogPath = getEnv("LOG_PATH", "logs/tiup_checker.log")

    // github token
    cfg.GitHubToken = getEnv("GH_TOKEN", "")

    // cron schedule
    cfg.CronSchedule = getEnv("CRON_SCHEDULE", "*/30 * * * *")
    cfg.EnableCron = getEnvBool("ENABLE_CRON", false)

    
    return cfg
}

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
    if value := os.Getenv(key); value != "" {
        if i, err := strconv.Atoi(value); err == nil {
            return i
        }
    }
    return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
    if value := os.Getenv(key); value != "" {
        if b, err := strconv.ParseBool(value); err == nil {
            return b
        }
    }
    return defaultValue
}