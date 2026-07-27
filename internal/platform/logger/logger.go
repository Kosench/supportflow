package logger

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

type Config struct {
	Level       string
	Pretty      bool
	Service     string
	Environment string
}

func New(out io.Writer, cfg Config) (zerolog.Logger, error) {
	if out == nil {
		return zerolog.Logger{}, fmt.Errorf("log output must not be nil")
	}

	level, err := zerolog.ParseLevel(strings.ToLower(cfg.Level))
	if err != nil {
		return zerolog.Logger{}, fmt.Errorf(
			"parse log level %q: %w",
			cfg.Level,
			err,
		)
	}

	if strings.TrimSpace(cfg.Service) == "" {
		return zerolog.Logger{}, fmt.Errorf("log service must not be empty")
	}

	if strings.TrimSpace(cfg.Environment) == "" {
		return zerolog.Logger{}, fmt.Errorf(
			"log environment must not be empty",
		)
	}

	zerolog.TimeFieldFormat = time.RFC3339Nano

	var writer io.Writer = zerolog.SyncWriter(out)
	if cfg.Pretty {
		writer = zerolog.SyncWriter(
			zerolog.ConsoleWriter{
				Out:        out,
				TimeFormat: time.RFC3339,
			},
		)
	}

	log := zerolog.New(writer).
		Level(level).
		With().
		Timestamp().
		Str("service", cfg.Service).
		Str("environment", cfg.Environment).
		Logger()

	return log, nil
}
