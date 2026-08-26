package handler

import (
	"net/http"

	"github.com/stan-ley-tech/medqueue/internal/domain"
	"github.com/stan-ley-tech/medqueue/internal/httpserver"
)

// Register is admin-only: new staff accounts are provisioned by an
// administrator, not by public self-signup, which matches how access to
// a clinical system should be granted.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeAndValidate(w, r, &req); err != nil {
		httpserver.WriteError(w, r, err)
		return
	}

	u, err := h.Auth.Register(r.Context(), req.Email, req.Password, req.Name, domain.Role(req.Role))
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusCreated, toUserResponse(*u))
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeAndValidate(w, r, &req); err != nil {
		httpserver.WriteError(w, r, err)
		return
	}

	session, err := h.Auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toSessionResponse(session))
}

func (h *Handler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := decodeAndValidate(w, r, &req); err != nil {
		httpserver.WriteError(w, r, err)
		return
	}

	session, err := h.Auth.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toSessionResponse(session))
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := decodeAndValidate(w, r, &req); err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	if err := h.Auth.Logout(r.Context(), req.RefreshToken); err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
