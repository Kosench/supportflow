package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Environment string

const (
	EnvironmentLocal      Environment = "local"
	EnvironmentTest       Environment = "test"
	EnvironmentStaging    Environment = "staging"
	EnvironmentProduction Environment = "production"
)

type Config struct {
	App AppConfig
	Log LogConfig
}

type AppConfig struct {
	Name            string
	Environment     Environment
	ShutdownTimeout time.Duration
}

type LogConfig struct {
	Level  string
	Pretty bool
}

func Load() (Config, error) {
	shutdownTimeout, err := durationFromEnv(
		"APP_SHUTDOWN_TIMEOUT",
		10*time.Second,
	)
	if err != nil {
		return Config{}, err
	}

	prettyLogs, err := boolFromEnv("LOG_PRETTY", false)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		App: AppConfig{
			Name: stringFromEnv("APP_NAME", "ticket-service"),
			Environment: Environment(
				stringFromEnv("APP_ENV", string(EnvironmentLocal)),
			),
			ShutdownTimeout: shutdownTimeout,
		},
		Log: LogConfig{
			Level:  stringFromEnv("LOG_LEVEL", "info"),
			Pretty: prettyLogs,
		},
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (cfg Config) Validate() error {
	if strings.TrimSpace(cfg.App.Name) == "" {
		return fmt.Errorf("APP_NAME must not be empty")
	}

	switch cfg.App.Environment {
	case EnvironmentLocal,
		EnvironmentTest,
		EnvironmentStaging,
		EnvironmentProduction:
	default:
		return fmt.Errorf(
			"APP_ENV has unsupported value %q",
			cfg.App.Environment,
		)
	}

	if cfg.App.ShutdownTimeout <= 0 {
		return fmt.Errorf("APP_SHUTDOWN_TIMEOUT must be positive")
	}

	return nil
}

func stringFromEnv(key, fallback string) string {
	value, exist := os.LookupEnv(key)
	if !exist || strings.TrimSpace(value) == "" {
		return fallback
	}

	return strings.TrimSpace(value)
}

func durationFromEnv(key string, fallback time.Duration) (time.Duration, error) {
	raw := stringFromEnv(key, fallback.String())

	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration: %w", key, err)
	}

	return value, nil
}

func boolFromEnv(key string, fallback bool) (bool, error) {
	raw := stringFromEnv(key, strconv.FormatBool(fallback))

	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be a valid boolean: %w", key, err)
	}

	return value, nil
}
