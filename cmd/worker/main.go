// Command worker runs background jobs outside the request/response
// cycle. Today that's the appointment reminder scan; it runs as its own
// process/container so it scales and restarts independently of the API.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/stan-ley-tech/medqueue/internal/config"
	"github.com/stan-ley-tech/medqueue/internal/db"
	"github.com/stan-ley-tech/medqueue/internal/logger"
	"github.com/stan-ley-tech/medqueue/internal/notifier"
	"github.com/stan-ley-tech/medqueue/internal/repository"
	"github.com/stan-ley-tech/medqueue/internal/service"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "worker:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log := logger.New(cfg.LogLevel, cfg.LogFormat)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, db.Options{
		URL:             cfg.DatabaseURL,
		MaxOpenConns:    int32(cfg.DBMaxOpenConns),
		MaxIdleConns:    int32(cfg.DBMaxIdleConns),
		ConnMaxLifetime: cfg.DBConnMaxLifetime,
	})
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	defer pool.Close()

	appointmentRepo := repository.NewAppointmentRepository(pool)
	reminderSvc := service.NewReminderService(appointmentRepo, notifier.NewLogNotifier(log), cfg.ReminderLookahead, log)

	log.Info("worker: starting reminder scan loop", "interval", cfg.ReminderScanInterval, "lookahead", cfg.ReminderLookahead)

	ticker := time.NewTicker(cfg.ReminderScanInterval)
	defer ticker.Stop()

	runScan(ctx, reminderSvc, log)

	for {
		select {
		case <-ctx.Done():
			log.Info("worker: shutdown signal received")
			return nil
		case <-ticker.C:
			runScan(ctx, reminderSvc, log)
		}
	}
}

func runScan(ctx context.Context, svc *service.ReminderService, log *slog.Logger) {
	sent, err := svc.RunOnce(ctx)
	if err != nil {
		log.Error("worker: reminder scan failed", "error", err)
		return
	}
	if sent > 0 {
		log.Info("worker: reminder scan completed", "sent", sent)
	}
}
