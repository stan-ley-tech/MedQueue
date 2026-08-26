package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/stan-ley-tech/medqueue/internal/domain"
	"github.com/stan-ley-tech/medqueue/internal/httpserver"
)

func (h *Handler) CreateDepartment(w http.ResponseWriter, r *http.Request) {
	var req createDepartmentRequest
	if err := decodeAndValidate(w, r, &req); err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	actor, err := actorFromRequest(r)
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}

	d := &domain.Department{Name: req.Name, Description: req.Description}
	if err := h.Departments.Create(r.Context(), actor, d); err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusCreated, toDepartmentResponse(*d))
}

func (h *Handler) GetDepartment(w http.ResponseWriter, r *http.Request) {
	d, err := h.Departments.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toDepartmentResponse(*d))
}

func (h *Handler) UpdateDepartment(w http.ResponseWriter, r *http.Request) {
	var req updateDepartmentRequest
	if err := decodeAndValidate(w, r, &req); err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	actor, err := actorFromRequest(r)
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}

	d := &domain.Department{ID: chi.URLParam(r, "id"), Name: req.Name, Description: req.Description, Active: req.Active}
	if err := h.Departments.Update(r.Context(), actor, d); err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toDepartmentResponse(*d))
}

func (h *Handler) ListDepartments(w http.ResponseWriter, r *http.Request) {
	filter := domain.DepartmentFilter{Search: r.URL.Query().Get("search"), Page: parsePage(r)}
	result, err := h.Departments.List(r.Context(), filter)
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}

	items := make([]departmentResponse, 0, len(result.Items))
	for _, d := range result.Items {
		items = append(items, toDepartmentResponse(d))
	}
	httpserver.WriteJSON(w, http.StatusOK, pagedResponse[departmentResponse]{
		Items: items, Total: result.Total, Limit: result.Limit, Offset: result.Offset,
	})
}
