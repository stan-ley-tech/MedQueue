// Package router assembles the HTTP route tree. It is the one place
// allowed to depend on both internal/handler and internal/httpserver, so
// neither of those has to depend on the other.
package router

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stan-ley-tech/medqueue/internal/auth"
	"github.com/stan-ley-tech/medqueue/internal/domain"
	"github.com/stan-ley-tech/medqueue/internal/handler"
	custommw "github.com/stan-ley-tech/medqueue/internal/httpserver/middleware"
	"github.com/stan-ley-tech/medqueue/internal/repository"
)

type Config struct {
	Handler            *handler.Handler
	Health             *handler.HealthHandler
	Tokens             *auth.TokenIssuer
	Idempotency        repository.IdempotencyRepository
	Redis              *redis.Client
	Log                *slog.Logger
	CORSAllowedOrigins []string
	RateLimitPerMinute int
	SwaggerSpecPath    string
}

func New(cfg Config) *chi.Mux {
	r := chi.NewRouter()

	r.Use(custommw.RequestID)
	r.Use(custommw.Recover(cfg.Log))
	r.Use(custommw.Logging(cfg.Log))
	r.Use(custommw.CORS(cfg.CORSAllowedOrigins))
	r.Use(custommw.RateLimit(cfg.Redis, cfg.RateLimitPerMinute, time.Minute))

	r.Get("/healthz", cfg.Health.Live)
	r.Get("/readyz", cfg.Health.Ready)

	if cfg.SwaggerSpecPath != "" {
		r.Get("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, cfg.SwaggerSpecPath)
		})
	}

	h := cfg.Handler
	authenticate := custommw.Authenticate(cfg.Tokens)
	idempotent := custommw.Idempotency(cfg.Idempotency)

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/login", h.Login)
			r.Post("/refresh", h.RefreshToken)
			r.Post("/logout", h.Logout)

			r.Group(func(r chi.Router) {
				r.Use(authenticate, custommw.RequireRole(domain.RoleAdmin))
				r.Post("/register", h.Register)
			})
		})

		r.Group(func(r chi.Router) {
			r.Use(authenticate)

			r.Get("/departments", h.ListDepartments)
			r.Get("/departments/{id}", h.GetDepartment)
			r.Get("/doctors", h.ListDoctors)
			r.Get("/doctors/{id}", h.GetDoctor)
			r.Get("/patients", h.ListPatients)
			r.Get("/patients/{id}", h.GetPatient)
			r.Get("/appointments", h.ListAppointments)
			r.Get("/appointments/{id}", h.GetAppointment)
			r.Get("/departments/{id}/queue", h.GetQueueSnapshot)
			r.Get("/ws/departments/{id}/queue", h.QueueEvents)

			r.Group(func(r chi.Router) {
				r.Use(custommw.RequireRole(domain.RoleAdmin))
				r.Post("/departments", h.CreateDepartment)
				r.Put("/departments/{id}", h.UpdateDepartment)
				r.Post("/doctors", h.CreateDoctor)
				r.Put("/doctors/{id}", h.UpdateDoctor)
				r.Get("/audit-logs", h.ListAuditLogs)
			})

			r.Group(func(r chi.Router) {
				r.Use(custommw.RequireRole(domain.RoleAdmin, domain.RoleFrontDesk))
				r.Post("/patients", h.CreatePatient)
				r.Put("/patients/{id}", h.UpdatePatient)

				r.With(idempotent).Post("/appointments", h.ScheduleAppointment)
				r.Put("/appointments/{id}", h.RescheduleAppointment)
				r.Post("/appointments/{id}/cancel", h.CancelAppointment)
				r.Post("/appointments/{id}/no-show", h.MarkAppointmentNoShow)
				r.With(idempotent).Post("/appointments/{id}/check-in", h.CheckIn)
			})

			r.Group(func(r chi.Router) {
				r.Use(custommw.RequireRole(domain.RoleAdmin, domain.RoleClinician))
				r.Post("/departments/{id}/queue/call-next", h.CallNext)
				r.Post("/queue/{id}/start", h.StartConsultation)
				r.Post("/queue/{id}/complete", h.CompleteConsultation)
				r.Post("/queue/{id}/requeue", h.RequeuePatient)
				r.Post("/queue/{id}/no-show", h.MarkQueueNoShow)
			})
		})
	})

	return r
}
