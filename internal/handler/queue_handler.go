package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/stan-ley-tech/medqueue/internal/domain"
	"github.com/stan-ley-tech/medqueue/internal/httpserver"
)

func (h *Handler) CheckIn(w http.ResponseWriter, r *http.Request) {
	var req checkInRequest
	if err := decodeAndValidate(w, r, &req); err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	actor, err := actorFromRequest(r)
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}

	priority := domain.PriorityNormal
	if req.Priority != nil {
		priority = domain.QueuePriority(*req.Priority)
	}

	entry, err := h.Queue.CheckIn(r.Context(), actor, chi.URLParam(r, "id"), priority)
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusCreated, toQueueEntryResponse(*entry))
}

func (h *Handler) CallNext(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFromRequest(r)
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}

	entry, err := h.Queue.CallNext(r.Context(), actor, chi.URLParam(r, "id"))
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	if entry == nil {
		httpserver.WriteJSON(w, http.StatusOK, map[string]any{"message": "no patients waiting"})
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toQueueEntryResponse(*entry))
}

func (h *Handler) GetQueueSnapshot(w http.ResponseWriter, r *http.Request) {
	snapshot, err := h.Queue.Snapshot(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toQueueSnapshotResponse(snapshot))
}

func (h *Handler) StartConsultation(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFromRequest(r)
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	entry, err := h.Queue.StartConsultation(r.Context(), actor, chi.URLParam(r, "id"))
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toQueueEntryResponse(*entry))
}

func (h *Handler) CompleteConsultation(w http.ResponseWriter, r *http.Request) {
	var req completeConsultationRequest
	if err := decodeAndValidate(w, r, &req); err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	actor, err := actorFromRequest(r)
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}

	entry, err := h.Queue.CompleteConsultation(r.Context(), actor, chi.URLParam(r, "id"), req.Notes)
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toQueueEntryResponse(*entry))
}

func (h *Handler) RequeuePatient(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFromRequest(r)
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	entry, err := h.Queue.Requeue(r.Context(), actor, chi.URLParam(r, "id"))
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toQueueEntryResponse(*entry))
}

func (h *Handler) MarkQueueNoShow(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFromRequest(r)
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	entry, err := h.Queue.MarkNoShow(r.Context(), actor, chi.URLParam(r, "id"))
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toQueueEntryResponse(*entry))
}
