package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	URL               string
	ApplicationName   string
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
	ConnectTimeout    time.Duration
}

func DefaultConfig(url, applicationName string) Config {
	return Config{
		URL:               url,
		ApplicationName:   applicationName,
		MaxConns:          10,
		MinConns:          1,
		MaxConnLifetime:   time.Hour,
		MaxConnIdleTime:   30 * time.Minute,
		HealthCheckPeriod: time.Minute,
		ConnectTimeout:    5 * time.Second,
	}
}
func Open(ctx context.Context, config Config) (*pgxpool.Pool, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	poolConfig, err := pgxpool.ParseConfig(config.URL)
	if err != nil {
		return nil, fmt.Errorf("parse PosrgresSQL URL: %w", err)
	}

	poolConfig.MaxConns = config.MaxConns
	poolConfig.MinConns = config.MinConns
	poolConfig.MaxConnLifetime = config.MaxConnLifetime
	poolConfig.MaxConnIdleTime = config.MaxConnIdleTime
	poolConfig.HealthCheckPeriod = config.HealthCheckPeriod
	poolConfig.ConnConfig.ConnectTimeout = config.ConnectTimeout
	poolConfig.ConnConfig.RuntimeParams["application_name"] =
		config.ApplicationName
	poolConfig.ConnConfig.RuntimeParams["timezone"] = "UTC"

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}

	return pool, nil
}

func (cfg Config) Validate() error {
	if strings.TrimSpace(cfg.URL) == "" {
		return fmt.Errorf("PostgreSQL URL must not be empty")
	}

	if strings.TrimSpace(cfg.ApplicationName) == "" {
		return fmt.Errorf("PostgreSQL application name must not be empty")
	}

	if cfg.MaxConns <= 0 {
		return fmt.Errorf("PostgreSQL max connections must be positive")
	}

	if cfg.MinConns < 0 || cfg.MinConns > cfg.MaxConns {
		return fmt.Errorf(
			"PostgreSQL min connections must be between 0 and max connections",
		)
	}

	for name, value := range map[string]time.Duration{
		"max connection lifetime":  cfg.MaxConnLifetime,
		"max connection idle time": cfg.MaxConnIdleTime,
		"health check period":      cfg.HealthCheckPeriod,
		"connect timeout":          cfg.ConnectTimeout,
	} {
		if value <= 0 {
			return fmt.Errorf("PostgreSQL %s must be positive", name)
		}
	}

	return nil
}
