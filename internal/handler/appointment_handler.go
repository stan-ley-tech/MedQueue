package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stan-ley-tech/medqueue/internal/apperr"
	"github.com/stan-ley-tech/medqueue/internal/domain"
	"github.com/stan-ley-tech/medqueue/internal/httpserver"
)

func (h *Handler) ScheduleAppointment(w http.ResponseWriter, r *http.Request) {
	var req scheduleAppointmentRequest
	if err := decodeAndValidate(w, r, &req); err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	actor, err := actorFromRequest(r)
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}

	scheduledAt, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		httpserver.WriteError(w, r, apperr.Validation("invalid scheduled_at", map[string]string{"scheduled_at": "must be RFC3339, e.g. 2026-01-15T09:30:00Z"}))
		return
	}

	a := &domain.Appointment{
		PatientID: req.PatientID, DoctorID: req.DoctorID, DepartmentID: req.DepartmentID,
		ScheduledAt: scheduledAt, Reason: req.Reason,
	}
	if err := h.Appointments.Schedule(r.Context(), actor, a); err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusCreated, toAppointmentResponse(*a))
}

func (h *Handler) GetAppointment(w http.ResponseWriter, r *http.Request) {
	a, err := h.Appointments.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toAppointmentResponse(*a))
}

func (h *Handler) RescheduleAppointment(w http.ResponseWriter, r *http.Request) {
	var req rescheduleAppointmentRequest
	if err := decodeAndValidate(w, r, &req); err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	actor, err := actorFromRequest(r)
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}

	newTime, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		httpserver.WriteError(w, r, apperr.Validation("invalid scheduled_at", map[string]string{"scheduled_at": "must be RFC3339"}))
		return
	}

	a, err := h.Appointments.Reschedule(r.Context(), actor, chi.URLParam(r, "id"), newTime)
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toAppointmentResponse(*a))
}

func (h *Handler) CancelAppointment(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFromRequest(r)
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	if err := h.Appointments.Cancel(r.Context(), actor, chi.URLParam(r, "id")); err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) MarkAppointmentNoShow(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFromRequest(r)
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	if err := h.Appointments.MarkNoShow(r.Context(), actor, chi.URLParam(r, "id")); err != nil {
		httpserver.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListAppointments(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := domain.AppointmentFilter{
		PatientID: q.Get("patient_id"), DoctorID: q.Get("doctor_id"), DepartmentID: q.Get("department_id"),
		Status: domain.AppointmentStatus(q.Get("status")), Page: parsePage(r),
	}
	if from := q.Get("from"); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			filter.From = &t
		}
	}
	if to := q.Get("to"); to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			filter.To = &t
		}
	}

	result, err := h.Appointments.List(r.Context(), filter)
	if err != nil {
		httpserver.WriteError(w, r, err)
		return
	}

	items := make([]appointmentResponse, 0, len(result.Items))
	for _, a := range result.Items {
		items = append(items, toAppointmentResponse(a))
	}
	httpserver.WriteJSON(w, http.StatusOK, pagedResponse[appointmentResponse]{
		Items: items, Total: result.Total, Limit: result.Limit, Offset: result.Offset,
	})
}
