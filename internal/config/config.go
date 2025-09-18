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

// Config represents application runtime configuration loaded from environment variables.
// Environment variables take precedence over values defined in .env file.
type Config struct {
	App struct {
		Name           string
		Env            string
		Port           int
		RequestTimeout time.Duration
		Version        string
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
	c.App.Version = getEnv("APP_VERSION", "0.1.0")

	c.Log.Level = strings.ToLower(getEnv("LOG_LEVEL", "info"))
	c.Log.Encoding = strings.ToLower(getEnv("LOG_ENCODING", "json"))

	c.CORS.AllowedOrigins = getEnvAsCSV("CORS_ALLOWED_ORIGINS", []string{"*"})
	c.CORS.AllowedMethods = getEnvAsCSV("CORS_ALLOWED_METHODS", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"})
	c.CORS.AllowedHeaders = getEnvAsCSV("CORS_ALLOWED_HEADERS", []string{"Authorization", "Content-Type"})

	if err := validate(c); err != nil {
		return nil, err
	}
	return c, nil
}

func validate(c *Config) error {
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

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
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
