package handler

import (
	"net/http"
	"time"

	"github.com/stan-ley-tech/medqueue/internal/domain"
	"github.com/stan-ley-tech/medqueue/internal/httpserver"
)

type auditLogResponse struct {
	ID         string         `json:"id"`
	ActorID    string         `json:"actor_id"`
	ActorRole  string         `json:"actor_role"`
	Action     string         `json:"action"`
	EntityType string         `json:"entity_type"`
	EntityID   string         `json:"entity_id"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

func (h *Handler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := domain.AuditFilter{
		EntityType: q.Get("entity_type"), EntityID: q.Get("entity_id"), ActorID: q.Get("actor_id"),
		Page: parsePage(r),
	}

	result, err := h.Audit.List(r.Context(), filter)
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}

	items := make([]auditLogResponse, 0, len(result.Items))
	for _, a := range result.Items {
		items = append(items, auditLogResponse{
			ID: a.ID, ActorID: a.ActorID, ActorRole: string(a.ActorRole), Action: a.Action,
			EntityType: a.EntityType, EntityID: a.EntityID, Metadata: a.Metadata, CreatedAt: a.CreatedAt,
		})
	}
	httpserver.WriteJSON(w, http.StatusOK, pagedResponse[auditLogResponse]{
		Items: items, Total: result.Total, Limit: result.Limit, Offset: result.Offset,
	})
}
