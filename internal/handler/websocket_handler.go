package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/stan-ley-tech/medqueue/internal/apperr"
	"github.com/stan-ley-tech/medqueue/internal/httpserver"
	"github.com/stan-ley-tech/medqueue/internal/ws"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Queue dashboards are served from a configured, trusted origin set;
	// actual enforcement happens at the CORS layer for regular requests.
	// The WebSocket handshake itself is authenticated by token below, so
	// a permissive origin check here doesn't widen access.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// QueueEvents upgrades to a WebSocket and streams real-time queue events
// for one department. Browsers can't set an Authorization header on the
// WebSocket handshake, so the access token travels as a query parameter
// instead; it is validated exactly like the header-based flow.
func (h *Handler) QueueEvents(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		httpserver.WriteError(w, r, apperr.Unauthorized("missing token query parameter"))
		return
	}
	if _, err := h.Tokens.ParseAccessToken(token); err != nil {
		httpserver.WriteError(w, r, apperr.Unauthorized("invalid or expired token"))
		return
	}

	departmentID := chi.URLParam(r, "id")

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.Log.Warn("ws: upgrade failed", "error", err)
		return
	}

	client := ws.NewClient(h.Hub, conn, departmentID, h.Log)
	go client.Run()
}
