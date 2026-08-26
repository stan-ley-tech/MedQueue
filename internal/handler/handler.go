// Package handler implements the HTTP transport layer: decoding and
// validating requests, invoking the relevant service, and translating
// the result (or error) into the API's JSON contract. Handlers hold no
// business logic themselves — that all lives in internal/service — so
// this package stays thin and easy to keep in sync with API.md.
package handler

import (
	"log/slog"

	"github.com/stan-ley-tech/medqueue/internal/auth"
	"github.com/stan-ley-tech/medqueue/internal/service"
	"github.com/stan-ley-tech/medqueue/internal/ws"
)

type Handler struct {
	Patients     *service.PatientService
	Doctors      *service.DoctorService
	Departments  *service.DepartmentService
	Appointments *service.AppointmentService
	Queue        *service.QueueService
	Auth         *service.AuthService
	Audit        *service.AuditService
	Hub          *ws.Hub
	Tokens       *auth.TokenIssuer
	Log          *slog.Logger
}

func New(
	patients *service.PatientService,
	doctors *service.DoctorService,
	departments *service.DepartmentService,
	appointments *service.AppointmentService,
	queue *service.QueueService,
	authSvc *service.AuthService,
	audit *service.AuditService,
	hub *ws.Hub,
	tokens *auth.TokenIssuer,
	log *slog.Logger,
) *Handler {
	return &Handler{
		Patients: patients, Doctors: doctors, Departments: departments,
		Appointments: appointments, Queue: queue, Auth: authSvc, Audit: audit,
		Hub: hub, Tokens: tokens, Log: log,
	}
}
