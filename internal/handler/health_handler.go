package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/stan-ley-tech/medqueue/internal/cache"
	"github.com/stan-ley-tech/medqueue/internal/db"
	"github.com/stan-ley-tech/medqueue/internal/httpserver"
)

type HealthHandler struct {
	db    *db.Pool
	cache *cache.Client
}

func NewHealthHandler(pool *db.Pool, cacheClient *cache.Client) *HealthHandler {
	return &HealthHandler{db: pool, cache: cacheClient}
}

// Live reports whether the process itself is running. It never touches a
// dependency, so a slow database doesn't cause the orchestrator to kill a
// perfectly healthy process.
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	httpserver.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready reports whether the service can actually serve traffic: both
// PostgreSQL and Redis must respond within a short timeout. Used as the
// Kubernetes/orchestrator readiness probe so traffic isn't routed here
// during startup or a dependency outage.
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	checks := map[string]string{}
	healthy := true

	if err := h.db.Ping(ctx); err != nil {
		checks["database"] = "unavailable"
		healthy = false
	} else {
		checks["database"] = "ok"
	}

	if err := h.cache.Ping(ctx).Err(); err != nil {
		checks["cache"] = "unavailable"
		healthy = false
	} else {
		checks["cache"] = "ok"
	}

	status := http.StatusOK
	if !healthy {
		status = http.StatusServiceUnavailable
	}
	httpserver.WriteJSON(w, status, map[string]any{"status": checks})
}
