package service

import (
	"context"
	"time"

	"github.com/stan-ley-tech/medqueue/internal/apperr"
	"github.com/stan-ley-tech/medqueue/internal/domain"
	"github.com/stan-ley-tech/medqueue/internal/repository"
)

type AppointmentService struct {
	repo         repository.AppointmentRepository
	doctors      repository.DoctorRepository
	audit        *AuditService
	slotDuration time.Duration
}

func NewAppointmentService(repo repository.AppointmentRepository, doctors repository.DoctorRepository, audit *AuditService, slotDuration time.Duration) *AppointmentService {
	return &AppointmentService{repo: repo, doctors: doctors, audit: audit, slotDuration: slotDuration}
}

func (s *AppointmentService) Schedule(ctx context.Context, actor Actor, a *domain.Appointment) error {
	if a.ScheduledAt.Before(time.Now().Add(-1 * time.Minute)) {
		return apperr.Validation("appointment cannot be scheduled in the past", map[string]string{"scheduled_at": "must be in the future"})
	}

	doctor, err := s.doctors.GetByID(ctx, a.DoctorID)
	if err != nil {
		return err
	}
	if !doctor.Active {
		return apperr.Validation("doctor is not active", map[string]string{"doctor_id": "this doctor cannot be booked"})
	}
	if doctor.DepartmentID != a.DepartmentID {
		return apperr.Validation("doctor does not belong to this department", map[string]string{"department_id": "doctor/department mismatch"})
	}

	overlapping, err := s.repo.ExistsOverlapping(ctx, a.DoctorID, a.ScheduledAt, s.slotDuration, "")
	if err != nil {
		return err
	}
	if overlapping {
		return apperr.Conflict("doctor already has an appointment within this time slot")
	}

	a.Status = domain.AppointmentScheduled
	if err := s.repo.Create(ctx, a); err != nil {
		return err
	}

	s.audit.Record(ctx, actor.UserID, actor.Role, "appointment.scheduled", "appointment", a.ID, map[string]any{
		"patient_id": a.PatientID, "doctor_id": a.DoctorID, "scheduled_at": a.ScheduledAt,
	})
	return nil
}

func (s *AppointmentService) Get(ctx context.Context, id string) (*domain.Appointment, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *AppointmentService) List(ctx context.Context, filter domain.AppointmentFilter) (domain.PagedResult[domain.Appointment], error) {
	items, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return domain.PagedResult[domain.Appointment]{}, err
	}
	return domain.PagedResult[domain.Appointment]{Items: items, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *AppointmentService) Reschedule(ctx context.Context, actor Actor, id string, newTime time.Time) (*domain.Appointment, error) {
	a, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if a.Status != domain.AppointmentScheduled {
		return nil, apperr.Conflict("only appointments still in scheduled status can be rescheduled")
	}
	if newTime.Before(time.Now()) {
		return nil, apperr.Validation("new time must be in the future", map[string]string{"scheduled_at": "must be in the future"})
	}

	overlapping, err := s.repo.ExistsOverlapping(ctx, a.DoctorID, newTime, s.slotDuration, a.ID)
	if err != nil {
		return nil, err
	}
	if overlapping {
		return nil, apperr.Conflict("doctor already has an appointment within this time slot")
	}

	a.ScheduledAt = newTime
	if err := s.repo.Update(ctx, a); err != nil {
		return nil, err
	}
	s.audit.Record(ctx, actor.UserID, actor.Role, "appointment.rescheduled", "appointment", a.ID, map[string]any{"scheduled_at": newTime})
	return a, nil
}

func (s *AppointmentService) Cancel(ctx context.Context, actor Actor, id string) error {
	return s.transition(ctx, actor, id, domain.AppointmentCancelled, "appointment.cancelled")
}

func (s *AppointmentService) MarkNoShow(ctx context.Context, actor Actor, id string) error {
	return s.transition(ctx, actor, id, domain.AppointmentNoShow, "appointment.no_show")
}

func (s *AppointmentService) transition(ctx context.Context, actor Actor, id string, to domain.AppointmentStatus, action string) error {
	a, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !a.Status.CanTransition(to) {
		return apperr.Conflict((&domain.InvalidTransitionError{From: a.Status, To: to}).Error())
	}
	if err := s.repo.UpdateStatus(ctx, id, to); err != nil {
		return err
	}
	s.audit.Record(ctx, actor.UserID, actor.Role, action, "appointment", id, map[string]any{"from": a.Status, "to": to})
	return nil
}
