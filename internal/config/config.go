package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppName            string
	HTTPAddr           string
	WorkerPollInterval time.Duration
	JWTSecret          string
	SeedAdminEmail     string
	SeedAdminPassword  string
	StorageDriver      string
	PostgresDSN        string
	PostgresAutoMigrate bool

	// MySQL 配置
	MySQLHost     string
	MySQLPort     int
	MySQLDatabase string
	MySQLUser     string
	MySQLPassword string
	MySQLDSN      string

	// LLM 配置
	LLMProvider  string
	LLMAPIKey    string
	LLMBaseURL   string
	LLMModel     string
}

func Load() Config {
	return Config{
		AppName:             env("APP_NAME", "go-agent-platform"),
		HTTPAddr:            env("HTTP_ADDR", ":8081"),
		WorkerPollInterval:  envDuration("WORKER_POLL_INTERVAL", 5*time.Second),
		JWTSecret:           env("JWT_SECRET", "dev-secret"),
		SeedAdminEmail:      env("SEED_ADMIN_EMAIL", "admin@example.com"),
		SeedAdminPassword:   env("SEED_ADMIN_PASSWORD", "ChangeMe123!"),
		StorageDriver:       env("STORAGE_DRIVER", "memory"),
		PostgresDSN:         env("POSTGRES_DSN", "postgres://agent:agent@127.0.0.1:5432/agent_platform?sslmode=disable"),
		PostgresAutoMigrate: envBool("POSTGRES_AUTO_MIGRATE", true),

		// MySQL 配置
		MySQLHost:     env("MYSQL_HOST", "localhost"),
		MySQLPort:     envInt("MYSQL_PORT", 3306),
		MySQLDatabase: env("MYSQL_DATABASE", "agent_platform_cloud"),
		MySQLUser:     env("MYSQL_USER", "agent"),
		MySQLPassword: env("MYSQL_PASSWORD", "agent123"),
		MySQLDSN:      env("MYSQL_DSN", ""),

		// LLM 配置
		LLMProvider:  env("LLM_PROVIDER", "openai"),
		LLMAPIKey:    env("LLM_API_KEY", ""),
		LLMBaseURL:   env("LLM_BASE_URL", "https://api.deepseek.com"),
		LLMModel:     env("LLM_MODEL", "deepseek-v4-flash"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return fallback
}
