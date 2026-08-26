package testutil

import (
	"log/slog"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stan-ley-tech/medqueue/internal/auth"
	"github.com/stan-ley-tech/medqueue/internal/cache"
	"github.com/stan-ley-tech/medqueue/internal/db"
	"github.com/stan-ley-tech/medqueue/internal/handler"
	"github.com/stan-ley-tech/medqueue/internal/repository"
	"github.com/stan-ley-tech/medqueue/internal/router"
	"github.com/stan-ley-tech/medqueue/internal/service"
	"github.com/stan-ley-tech/medqueue/internal/ws"
)

// NewTestServer wires the full application stack (every repository,
// service, and the HTTP router) against a real migrated test database and
// Redis instance, and returns it as an httptest.Server so e2e tests can
// drive it exactly like a deployed API.
func NewTestServer(t *testing.T, pool *db.Pool, redisClient *cache.Client) *httptest.Server {
	t.Helper()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	userRepo := repository.NewUserRepository(pool)
	patientRepo := repository.NewPatientRepository(pool)
	doctorRepo := repository.NewDoctorRepository(pool)
	departmentRepo := repository.NewDepartmentRepository(pool)
	appointmentRepo := repository.NewAppointmentRepository(pool)
	queueRepo := repository.NewQueueRepository(pool)
	auditRepo := repository.NewAuditRepository(pool)
	idempotencyRepo := repository.NewIdempotencyRepository(pool)
	refreshTokenRepo := repository.NewRefreshTokenRepository(pool)

	tokens := auth.NewTokenIssuer("test-access-secret", "test-refresh-secret", 15*time.Minute, time.Hour)
	queueCache := cache.NewQueueCache(redisClient)

	auditSvc := service.NewAuditService(auditRepo, log)
	patientSvc := service.NewPatientService(patientRepo, auditSvc)
	doctorSvc := service.NewDoctorService(doctorRepo, auditSvc)
	departmentSvc := service.NewDepartmentService(departmentRepo, auditSvc)
	appointmentSvc := service.NewAppointmentService(appointmentRepo, doctorRepo, auditSvc, 15*time.Minute)
	queueSvc := service.NewQueueService(pool, queueRepo, appointmentRepo, queueCache, auditSvc, log)
	authSvc := service.NewAuthService(userRepo, doctorRepo, refreshTokenRepo, tokens, auditSvc)

	hub := ws.NewHub(queueCache, log)

	h := handler.New(patientSvc, doctorSvc, departmentSvc, appointmentSvc, queueSvc, authSvc, auditSvc, hub, tokens, log)
	healthHandler := handler.NewHealthHandler(pool, redisClient)

	mux := router.New(router.Config{
		Handler:            h,
		Health:             healthHandler,
		Tokens:             tokens,
		Idempotency:        idempotencyRepo,
		Redis:              redisClient.Client,
		Log:                log,
		CORSAllowedOrigins: []string{"http://localhost:3000"},
		RateLimitPerMinute: 100000, // effectively unlimited for tests
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}
