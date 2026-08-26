// Package config loads application configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv  string
	AppName string

	HTTPPort        string
	ShutdownTimeout time.Duration

	DatabaseURL          string
	DBMaxOpenConns       int
	DBMaxIdleConns       int
	DBConnMaxLifetime    time.Duration

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	JWTAccessSecret  string
	JWTRefreshSecret string
	JWTAccessTTL     time.Duration
	JWTRefreshTTL    time.Duration

	RateLimitRequestsPerMinute int
	RateLimitBurst             int

	LogLevel  string
	LogFormat string

	ReminderScanInterval    time.Duration
	ReminderLookahead       time.Duration

	CORSAllowedOrigins []string
}

// Load reads configuration from environment variables, applying sane
// defaults for anything not set. It fails fast on required secrets so a
// misconfigured deployment never starts serving traffic.
func Load() (*Config, error) {
	cfg := &Config{
		AppEnv:  getEnv("APP_ENV", "development"),
		AppName: getEnv("APP_NAME", "medqueue"),

		HTTPPort:        getEnv("HTTP_PORT", "8080"),
		ShutdownTimeout: getEnvDuration("SHUTDOWN_TIMEOUT_SECONDS", 15*time.Second, time.Second),

		DatabaseURL:       getEnv("DATABASE_URL", ""),
		DBMaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 10),
		DBConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME_MINUTES", 30*time.Minute, time.Minute),

		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvInt("REDIS_DB", 0),

		JWTAccessSecret:  getEnv("JWT_ACCESS_SECRET", ""),
		JWTRefreshSecret: getEnv("JWT_REFRESH_SECRET", ""),
		JWTAccessTTL:     getEnvDuration("JWT_ACCESS_TTL_MINUTES", 15*time.Minute, time.Minute),
		JWTRefreshTTL:    getEnvDuration("JWT_REFRESH_TTL_HOURS", 168*time.Hour, time.Hour),

		RateLimitRequestsPerMinute: getEnvInt("RATE_LIMIT_REQUESTS_PER_MINUTE", 120),
		RateLimitBurst:             getEnvInt("RATE_LIMIT_BURST", 30),

		LogLevel:  getEnv("LOG_LEVEL", "info"),
		LogFormat: getEnv("LOG_FORMAT", "json"),

		ReminderScanInterval: getEnvDuration("REMINDER_SCAN_INTERVAL_SECONDS", 60*time.Second, time.Second),
		ReminderLookahead:    getEnvDuration("REMINDER_LOOKAHEAD_MINUTES", 60*time.Minute, time.Minute),

		CORSAllowedOrigins: strings.Split(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000"), ","),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("config: DATABASE_URL is required")
	}
	if c.AppEnv == "production" {
		if c.JWTAccessSecret == "" || c.JWTRefreshSecret == "" {
			return fmt.Errorf("config: JWT secrets are required in production")
		}
		if c.JWTAccessSecret == c.JWTRefreshSecret {
			return fmt.Errorf("config: JWT access and refresh secrets must differ")
		}
	}
	if c.JWTAccessSecret == "" {
		c.JWTAccessSecret = "dev-access-secret-not-for-production"
	}
	if c.JWTRefreshSecret == "" {
		c.JWTRefreshSecret = "dev-refresh-secret-not-for-production"
	}
	return nil
}

func (c *Config) IsProduction() bool {
	return c.AppEnv == "production"
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvDuration(key string, fallback time.Duration, unit time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return time.Duration(n) * unit
}
