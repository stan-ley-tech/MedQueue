// Command api runs the MedQueue HTTP server: REST endpoints, WebSocket
// queue events, and the health/readiness probes. See cmd/worker for the
// separate background reminder process.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/stan-ley-tech/medqueue/internal/auth"
	"github.com/stan-ley-tech/medqueue/internal/cache"
	"github.com/stan-ley-tech/medqueue/internal/config"
	"github.com/stan-ley-tech/medqueue/internal/db"
	"github.com/stan-ley-tech/medqueue/internal/handler"
	"github.com/stan-ley-tech/medqueue/internal/logger"
	"github.com/stan-ley-tech/medqueue/internal/repository"
	"github.com/stan-ley-tech/medqueue/internal/router"
	"github.com/stan-ley-tech/medqueue/internal/service"
	"github.com/stan-ley-tech/medqueue/internal/ws"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "api:", err)
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

	redisClient, err := cache.Connect(ctx, cache.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisDB})
	if err != nil {
		return fmt.Errorf("connect redis: %w", err)
	}
	defer redisClient.Close()

	// Repositories
	userRepo := repository.NewUserRepository(pool)
	patientRepo := repository.NewPatientRepository(pool)
	doctorRepo := repository.NewDoctorRepository(pool)
	departmentRepo := repository.NewDepartmentRepository(pool)
	appointmentRepo := repository.NewAppointmentRepository(pool)
	queueRepo := repository.NewQueueRepository(pool)
	auditRepo := repository.NewAuditRepository(pool)
	idempotencyRepo := repository.NewIdempotencyRepository(pool)
	refreshTokenRepo := repository.NewRefreshTokenRepository(pool)

	// Cross-cutting infrastructure
	tokens := auth.NewTokenIssuer(cfg.JWTAccessSecret, cfg.JWTRefreshSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)
	queueCache := cache.NewQueueCache(redisClient)

	// Services
	auditSvc := service.NewAuditService(auditRepo, log)
	patientSvc := service.NewPatientService(patientRepo, auditSvc)
	doctorSvc := service.NewDoctorService(doctorRepo, auditSvc)
	departmentSvc := service.NewDepartmentService(departmentRepo, auditSvc)
	appointmentSvc := service.NewAppointmentService(appointmentRepo, doctorRepo, auditSvc, cfg.AppointmentSlotDuration)
	queueSvc := service.NewQueueService(pool, queueRepo, appointmentRepo, queueCache, auditSvc, log)
	authSvc := service.NewAuthService(userRepo, doctorRepo, refreshTokenRepo, tokens, auditSvc)

	// Real-time hub: one goroutine, subscribed to Redis for the life of
	// the process, fanning out to whatever WebSocket clients are attached
	// at the moment an event arrives.
	hub := ws.NewHub(queueCache, log)
	go hub.Run(ctx)

	h := handler.New(patientSvc, doctorSvc, departmentSvc, appointmentSvc, queueSvc, authSvc, auditSvc, hub, tokens, log)
	healthHandler := handler.NewHealthHandler(pool, redisClient)

	mux := router.New(router.Config{
		Handler:            h,
		Health:             healthHandler,
		Tokens:             tokens,
		Idempotency:        idempotencyRepo,
		Redis:              redisClient.Client,
		Log:                log,
		CORSAllowedOrigins: cfg.CORSAllowedOrigins,
		RateLimitPerMinute: cfg.RateLimitRequestsPerMinute,
		SwaggerSpecPath:    "docs/openapi.yaml",
	})

	srv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Info("api: listening", "port", cfg.HTTPPort, "env", cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
		close(serveErr)
	}()

	select {
	case err := <-serveErr:
		if err != nil {
			return fmt.Errorf("http server: %w", err)
		}
	case <-ctx.Done():
		log.Info("api: shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("api: graceful shutdown failed", "error", err)
		return err
	}

	log.Info("api: shutdown complete")
	return nil
}
