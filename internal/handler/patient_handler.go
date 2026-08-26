package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stan-ley-tech/medqueue/internal/apperr"
	"github.com/stan-ley-tech/medqueue/internal/domain"
	"github.com/stan-ley-tech/medqueue/internal/httpserver"
)

const dateLayout = "2006-01-02"

func (h *Handler) CreatePatient(w http.ResponseWriter, r *http.Request) {
	var req createPatientRequest
	if err := decodeAndValidate(w, r, &req); err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	actor, err := actorFromRequest(r)
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}

	dob, err := time.Parse(dateLayout, req.DateOfBirth)
	if err != nil {
		httpserver.WriteError(w, r, apperr.Validation("invalid date of birth", map[string]string{"date_of_birth": "must be YYYY-MM-DD"}))
		return
	}

	p := &domain.Patient{
		MedicalRecordNumber: req.MedicalRecordNumber, FirstName: req.FirstName, LastName: req.LastName,
		DateOfBirth: dob, Sex: req.Sex, Phone: req.Phone, Email: req.Email, Address: req.Address,
	}
	if err := h.Patients.Create(r.Context(), actor, p); err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusCreated, toPatientResponse(*p))
}

func (h *Handler) GetPatient(w http.ResponseWriter, r *http.Request) {
	p, err := h.Patients.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toPatientResponse(*p))
}

func (h *Handler) UpdatePatient(w http.ResponseWriter, r *http.Request) {
	var req updatePatientRequest
	if err := decodeAndValidate(w, r, &req); err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	actor, err := actorFromRequest(r)
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}

	dob, err := time.Parse(dateLayout, req.DateOfBirth)
	if err != nil {
		httpserver.WriteError(w, r, apperr.Validation("invalid date of birth", map[string]string{"date_of_birth": "must be YYYY-MM-DD"}))
		return
	}

	p := &domain.Patient{
		ID: chi.URLParam(r, "id"), FirstName: req.FirstName, LastName: req.LastName,
		DateOfBirth: dob, Sex: req.Sex, Phone: req.Phone, Email: req.Email, Address: req.Address,
	}
	if err := h.Patients.Update(r.Context(), actor, p); err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toPatientResponse(*p))
}

func (h *Handler) ListPatients(w http.ResponseWriter, r *http.Request) {
	filter := domain.PatientFilter{Search: r.URL.Query().Get("search"), Page: parsePage(r)}
	result, err := h.Patients.List(r.Context(), filter)
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}

	items := make([]patientResponse, 0, len(result.Items))
	for _, p := range result.Items {
		items = append(items, toPatientResponse(p))
	}
	httpserver.WriteJSON(w, http.StatusOK, pagedResponse[patientResponse]{
		Items: items, Total: result.Total, Limit: result.Limit, Offset: result.Offset,
	})
}
