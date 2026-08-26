package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/stan-ley-tech/medqueue/internal/domain"
	"github.com/stan-ley-tech/medqueue/internal/httpserver"
)

func (h *Handler) CreateDoctor(w http.ResponseWriter, r *http.Request) {
	var req createDoctorRequest
	if err := decodeAndValidate(w, r, &req); err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	actor, err := actorFromRequest(r)
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}

	d := &domain.Doctor{Name: req.Name, Specialty: req.Specialty, DepartmentID: req.DepartmentID}
	if req.UserID != "" {
		d.UserID = &req.UserID
	}
	if err := h.Doctors.Create(r.Context(), actor, d); err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusCreated, toDoctorResponse(*d))
}

func (h *Handler) GetDoctor(w http.ResponseWriter, r *http.Request) {
	d, err := h.Doctors.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toDoctorResponse(*d))
}

func (h *Handler) UpdateDoctor(w http.ResponseWriter, r *http.Request) {
	var req updateDoctorRequest
	if err := decodeAndValidate(w, r, &req); err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	actor, err := actorFromRequest(r)
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}

	d := &domain.Doctor{
		ID: chi.URLParam(r, "id"), Name: req.Name, Specialty: req.Specialty,
		DepartmentID: req.DepartmentID, Active: req.Active,
	}
	if err := h.Doctors.Update(r.Context(), actor, d); err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toDoctorResponse(*d))
}

func (h *Handler) ListDoctors(w http.ResponseWriter, r *http.Request) {
	filter := domain.DoctorFilter{DepartmentID: r.URL.Query().Get("department_id"), Page: parsePage(r)}
	if v := r.URL.Query().Get("active"); v != "" {
		active := v == "true"
		filter.Active = &active
	}

	result, err := h.Doctors.List(r.Context(), filter)
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}

	items := make([]doctorResponse, 0, len(result.Items))
	for _, d := range result.Items {
		items = append(items, toDoctorResponse(d))
	}
	httpserver.WriteJSON(w, http.StatusOK, pagedResponse[doctorResponse]{
		Items: items, Total: result.Total, Limit: result.Limit, Offset: result.Offset,
	})
}
