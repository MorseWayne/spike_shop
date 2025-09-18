package config

import (
	"os"
	"testing"
)

func withEnv(key, value string, fn func()) {
	orig, had := os.LookupEnv(key)
	os.Setenv(key, value)
	defer func() {
		if had {
			os.Setenv(key, orig)
		} else {
			os.Unsetenv(key)
		}
	}()
	fn()
}

func TestLoad_DefaultsAndValidation_OK(t *testing.T) {
	// ensure minimal env; rely on defaults
	os.Unsetenv("APP_ENV")
	os.Unsetenv("APP_PORT")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.App.Port == 0 || cfg.App.RequestTimeout <= 0 {
		t.Fatalf("unexpected defaults: port=%d timeout=%s", cfg.App.Port, cfg.App.RequestTimeout)
	}
}

func TestLoad_InvalidEnv_ShouldError(t *testing.T) {
	withEnv("APP_ENV", "invalid", func() {
		if _, err := Load(); err == nil {
			t.Fatalf("expected error for invalid APP_ENV")
		}
	})
}

func TestLoad_InvalidPort_ShouldError(t *testing.T) {
	withEnv("APP_PORT", "70000", func() {
		if _, err := Load(); err == nil {
			t.Fatalf("expected error for invalid APP_PORT")
		}
	})
}
