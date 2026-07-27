package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadUsesDefaults(t *testing.T) {
	clearEnvironment(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.App.Name != "ticket-service" {
		t.Errorf("unexpected app name: %q", cfg.App.Name)
	}

	if cfg.App.Environment != EnvironmentLocal {
		t.Errorf("unexpected environment: %q", cfg.App.Environment)
	}

	if cfg.App.ShutdownTimeout != 10*time.Second {
		t.Errorf(
			"unexpected shutdown timeout: %s",
			cfg.App.ShutdownTimeout,
		)
	}

	if cfg.Log.Level != "info" {
		t.Errorf("unexpected log level: %q", cfg.Log.Level)
	}

	if cfg.Log.Pretty {
		t.Error("pretty logs must be disabled by default")
	}
}

func TestLoadReadsEnvironment(t *testing.T) {
	clearEnvironment(t)
	t.Setenv("APP_NAME", "custom-ticket-service")
	t.Setenv("APP_ENV", "staging")
	t.Setenv("APP_SHUTDOWN_TIMEOUT", "25s")
	t.Setenv("LOG_LEVEL", "warn")
	t.Setenv("LOG_PRETTY", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.App.Name != "custom-ticket-service" {
		t.Errorf("unexpected app name: %q", cfg.App.Name)
	}

	if cfg.App.Environment != EnvironmentStaging {
		t.Errorf("unexpected environment: %q", cfg.App.Environment)
	}

	if cfg.App.ShutdownTimeout != 25*time.Second {
		t.Errorf(
			"unexpected shutdown timeout: %s",
			cfg.App.ShutdownTimeout,
		)
	}

	if cfg.Log.Level != "warn" {
		t.Errorf("unexpected log level: %q", cfg.Log.Level)
	}

	if !cfg.Log.Pretty {
		t.Error("pretty logs must be enabled")
	}
}

func TestLoadRejectsInvalidDuration(t *testing.T) {
	clearEnvironment(t)
	t.Setenv("APP_SHUTDOWN_TIMEOUT", "ten seconds")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error is nil")
	}

	if !strings.Contains(err.Error(), "APP_SHUTDOWN_TIMEOUT") {
		t.Errorf("error does not name invalid variable: %v", err)
	}
}

func TestLoadRejectsUnknownEnvironment(t *testing.T) {
	clearEnvironment(t)
	t.Setenv("APP_ENV", "demo")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error is nil")
	}

	if !strings.Contains(err.Error(), "APP_ENV") {
		t.Errorf("error does not name invalid variable: %v", err)
	}
}

func TestLoadRejectsInvalidBoolean(t *testing.T) {
	clearEnvironment(t)
	t.Setenv("LOG_PRETTY", "sometimes")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error is nil")
	}

	if !strings.Contains(err.Error(), "LOG_PRETTY") {
		t.Errorf("error does not name invalid variable: %v", err)
	}
}

func clearEnvironment(t *testing.T) {
	t.Helper()

	for _, key := range []string{
		"APP_NAME",
		"APP_ENV",
		"APP_SHUTDOWN_TIMEOUT",
		"LOG_LEVEL",
		"LOG_PRETTY",
	} {
		t.Setenv(key, "")
	}
}
