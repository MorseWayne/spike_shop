// Package config 负责应用配置的加载（环境变量优先）、默认值设定与启动前校验。
// 约定：环境变量优先于 .env 文件；非法配置会在启动阶段直接失败并给出清晰错误。
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config 表示应用运行时配置，来源于环境变量（若存在 .env 会被优先装载，但不会覆盖已存在的环境变量）。
// 建议的环境变量（与默认值）：
//   - APP_NAME=spike-server
//   - APP_ENV=dev|test|prod（默认 dev）
//   - APP_PORT（默认 8080）
//   - REQUEST_TIMEOUT_MS（默认 5000）
//   - LOG_LEVEL=debug|info|warn|error（默认 info）
//   - LOG_ENCODING=json|console（默认 json）
//   - CORS_ALLOWED_ORIGINS, CORS_ALLOWED_METHODS, CORS_ALLOWED_HEADERS（CSV）
type Config struct {
	App struct {
		Name            string
		Env             string
		Port            int
		RequestTimeout  time.Duration
		Version         string
		ShutdownTimeout time.Duration
	}
	Log struct {
		Level    string
		Encoding string
	}
	CORS struct {
		AllowedOrigins []string
		AllowedMethods []string
		AllowedHeaders []string
	}
	Database struct {
		Host     string
		Port     int
		User     string
		Password string
		DBName   string
	}
	JWT struct {
		Secret          string
		AccessTokenTTL  time.Duration
		RefreshTokenTTL time.Duration
	}
	Migrations struct {
		Dir string
	}
	Cache struct {
		Enabled bool
		TTL     time.Duration
		Type    string // "memory" 或 "redis"
	}
	Redis struct {
		Host     string
		Port     int
		Password string
		DB       int
	}
}

// Load reads configuration from the environment (optionally loading a .env file if present),
// applies defaults, and validates the result.
func Load() (*Config, error) {
	// Load .env if present. Environment variables that are already set will NOT be overridden.
	_ = godotenv.Load()

	c := &Config{}

	// Defaults
	c.App.Name = getEnv("APP_NAME", "spike-server")
	c.App.Env = getEnv("APP_ENV", "dev")
	c.App.Port = getEnvAsInt("APP_PORT", 8080)
	c.App.RequestTimeout = getEnvAsDurationMs("REQUEST_TIMEOUT_MS", 5000)
	c.App.ShutdownTimeout = getEnvAsDurationMs("SHUTDOWN_TIMEOUT_MS", 5000)
	c.App.Version = getEnv("APP_VERSION", "0.1.0")

	c.Log.Level = strings.ToLower(getEnv("LOG_LEVEL", "info"))
	c.Log.Encoding = strings.ToLower(getEnv("LOG_ENCODING", "console"))

	c.CORS.AllowedOrigins = getEnvAsCSV("CORS_ALLOWED_ORIGINS", []string{"*"})
	c.CORS.AllowedMethods = getEnvAsCSV("CORS_ALLOWED_METHODS", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"})
	c.CORS.AllowedHeaders = getEnvAsCSV("CORS_ALLOWED_HEADERS", []string{"Authorization", "Content-Type"})

	c.Database.Host = getEnv("MYSQL_HOST", "localhost")
	c.Database.Port = getEnvAsInt("MYSQL_PORT", 3306)
	c.Database.User = getEnv("MYSQL_USER", "spike")
	c.Database.Password = getEnv("MYSQL_PASSWORD", "spike")
	c.Database.DBName = getEnv("MYSQL_DB", "spike")

	c.JWT.Secret = getEnv("JWT_SECRET", "change_me_in_production")
	c.JWT.AccessTokenTTL = getEnvAsDuration("ACCESS_TOKEN_TTL", "15m")
	c.JWT.RefreshTokenTTL = getEnvAsDuration("REFRESH_TOKEN_TTL", "168h")

	// 数据库迁移配置
	c.Migrations.Dir = getEnv("MIGRATIONS_DIR", "migrations")

	// 缓存配置
	c.Cache.Enabled = getEnvAsBool("CACHE_ENABLED", true)
	c.Cache.TTL = getEnvAsDuration("CACHE_TTL", "5m")
	c.Cache.Type = getEnv("CACHE_TYPE", "memory")

	// Redis配置
	c.Redis.Host = getEnv("REDIS_HOST", "localhost")
	c.Redis.Port = getEnvAsInt("REDIS_PORT", 6379)
	c.Redis.Password = getEnv("REDIS_PASSWORD", "")
	c.Redis.DB = getEnvAsInt("REDIS_DB", 0)

	if err := validate(c); err != nil {
		return nil, err
	}
	return c, nil
}

func validate(c *Config) error {
	var errs []string

	errs = append(errs, validateApp(c)...)
	errs = append(errs, validateLog(c)...)
	errs = append(errs, validateDatabase(c)...)
	errs = append(errs, validateJWT(c)...)

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func validateApp(c *Config) []string {
	var errs []string

	switch c.App.Env {
	case "dev", "test", "prod":
		// ok
	default:
		errs = append(errs, fmt.Sprintf("APP_ENV must be one of dev|test|prod, got %q", c.App.Env))
	}

	if c.App.Port < 1 || c.App.Port > 65535 {
		errs = append(errs, fmt.Sprintf("APP_PORT must be in range 1..65535, got %d", c.App.Port))
	}

	if c.App.RequestTimeout <= 0 {
		errs = append(errs, fmt.Sprintf("REQUEST_TIMEOUT_MS must be > 0, got %s", c.App.RequestTimeout))
	}

	return errs
}

func validateLog(c *Config) []string {
	var errs []string

	switch c.Log.Level {
	case "debug", "info", "warn", "error":
		// ok
	default:
		errs = append(errs, fmt.Sprintf("LOG_LEVEL must be one of debug|info|warn|error, got %q", c.Log.Level))
	}

	switch c.Log.Encoding {
	case "json", "console":
		// ok
	default:
		errs = append(errs, fmt.Sprintf("LOG_ENCODING must be one of json|console, got %q", c.Log.Encoding))
	}

	return errs
}

func validateDatabase(c *Config) []string {
	var errs []string

	if strings.TrimSpace(c.Database.Host) == "" {
		errs = append(errs, "MYSQL_HOST cannot be empty")
	}
	if c.Database.Port < 1 || c.Database.Port > 65535 {
		errs = append(errs, fmt.Sprintf("MYSQL_PORT must be in range 1..65535, got %d", c.Database.Port))
	}
	if strings.TrimSpace(c.Database.User) == "" {
		errs = append(errs, "MYSQL_USER cannot be empty")
	}
	if strings.TrimSpace(c.Database.DBName) == "" {
		errs = append(errs, "MYSQL_DB cannot be empty")
	}

	return errs
}

func validateJWT(c *Config) []string {
	var errs []string

	if strings.TrimSpace(c.JWT.Secret) == "" || c.JWT.Secret == "change_me_in_production" {
		if c.App.Env == "prod" {
			errs = append(errs, "JWT_SECRET must be set to a secure value in production")
		}
	}
	if c.JWT.AccessTokenTTL <= 0 {
		errs = append(errs, fmt.Sprintf("ACCESS_TOKEN_TTL must be > 0, got %s", c.JWT.AccessTokenTTL))
	}
	if c.JWT.RefreshTokenTTL <= 0 {
		errs = append(errs, fmt.Sprintf("REFRESH_TOKEN_TTL must be > 0, got %s", c.JWT.RefreshTokenTTL))
	}

	return errs
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return def
}

func getEnvAsInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return i
		}
	}
	return def
}

func getEnvAsDurationMs(key string, defMs int) time.Duration {
	ms := getEnvAsInt(key, defMs)
	return time.Duration(ms) * time.Millisecond
}

func getEnvAsDuration(key, def string) time.Duration {
	v := getEnv(key, def)
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	if d, err := time.ParseDuration(def); err == nil {
		return d
	}
	return 15 * time.Minute // fallback
}

func getEnvAsCSV(key string, def []string) []string {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return def
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return def
	}
	return out
}

func getEnvAsBool(key string, def bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return def
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return b
}
