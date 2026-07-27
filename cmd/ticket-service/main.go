package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/Kosench/supportflow/internal/platform/buildinfo"
	"github.com/Kosench/supportflow/internal/platform/config"
	applog "github.com/Kosench/supportflow/internal/platform/logger"
)

func main() {

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "ticket-service failed: %v\n", err)
		os.Exit(1)
	}

}
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log, err := applog.New(os.Stdout, applog.Config{
		Level:       cfg.Log.Level,
		Pretty:      cfg.Log.Pretty,
		Service:     cfg.App.Name,
		Environment: string(cfg.App.Environment),
	})
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}

	build := buildinfo.Current()

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	log.Info().
		Str("version", build.Version).
		Str("commit", build.Commit).
		Str("build_time", build.BuildTime).
		Str("go_version", runtime.Version()).
		Msg("ticket service started")

	<-ctx.Done()

	log.Info().
		Dur("shutdown_timeout", cfg.App.ShutdownTimeout).
		Msg("shutdown signal received")

	return nil
}
