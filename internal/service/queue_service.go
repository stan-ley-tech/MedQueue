package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/stan-ley-tech/medqueue/internal/apperr"
	"github.com/stan-ley-tech/medqueue/internal/cache"
	"github.com/stan-ley-tech/medqueue/internal/db"
	"github.com/stan-ley-tech/medqueue/internal/domain"
	"github.com/stan-ley-tech/medqueue/internal/repository"
)

// QueueService owns the check-in -> queue -> call -> consult -> complete
// lifecycle. Every mutation runs inside a PostgreSQL transaction (via
// pool.WithTx) so the appointment status and the queue entry status move
// together; the Redis-backed cache is only updated, and events only
// published, after that transaction has committed, so cached/broadcast
// state never gets ahead of the database.
type QueueService struct {
	pool         *db.Pool
	queue        repository.QueueRepository
	appointments repository.AppointmentRepository
	cache        *cache.QueueCache
	audit        *AuditService
	log          *slog.Logger
}

func NewQueueService(pool *db.Pool, queue repository.QueueRepository, appointments repository.AppointmentRepository, qc *cache.QueueCache, audit *AuditService, log *slog.Logger) *QueueService {
	return &QueueService{pool: pool, queue: queue, appointments: appointments, cache: qc, audit: audit, log: log}
}

func (s *QueueService) CheckIn(ctx context.Context, actor Actor, appointmentID string, priority domain.QueuePriority) (*domain.QueueEntry, error) {
	if !priority.Valid() {
		return nil, apperr.Validation("invalid priority", map[string]string{"priority": "must be 0 (normal), 1 (urgent), or 2 (emergency)"})
	}

	var entry *domain.QueueEntry
	err := s.pool.WithTx(ctx, func(ctx context.Context) error {
		appt, err := s.appointments.GetByID(ctx, appointmentID)
		if err != nil {
			return err
		}
		if !appt.Status.CanTransition(domain.AppointmentCheckedIn) {
			return apperr.Conflict((&domain.InvalidTransitionError{From: appt.Status, To: domain.AppointmentCheckedIn}).Error())
		}
		if err := s.appointments.UpdateStatus(ctx, appt.ID, domain.AppointmentCheckedIn); err != nil {
			return err
		}
		if err := s.appointments.UpdateStatus(ctx, appt.ID, domain.AppointmentInQueue); err != nil {
			return err
		}

		entry = &domain.QueueEntry{
			AppointmentID: appt.ID,
			PatientID:     appt.PatientID,
			DepartmentID:  appt.DepartmentID,
			Priority:      priority,
		}
		if err := s.queue.Create(ctx, entry); err != nil {
			return err
		}
		entry.PatientName = appt.PatientName

		s.audit.Record(ctx, actor.UserID, actor.Role, "patient.checked_in", "appointment", appt.ID, map[string]any{
			"queue_entry_id": entry.ID, "priority": priority,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.publishAndInvalidate(ctx, entry.DepartmentID, cache.EventPatientCheckedIn, entry)
	return entry, nil
}

// CallNext claims the next waiting patient in departmentID on behalf of
// the calling clinician. See QueueRepository.CallNext for the concurrency
// guarantee: concurrent calls (two clinicians, or two API replicas) never
// receive the same patient. Returns (nil, nil) when the queue is empty.
func (s *QueueService) CallNext(ctx context.Context, actor Actor, departmentID string) (*domain.QueueEntry, error) {
	var doctorID *string
	if actor.DoctorID != "" {
		doctorID = &actor.DoctorID
	}

	var entry *domain.QueueEntry
	err := s.pool.WithTx(ctx, func(ctx context.Context) error {
		e, err := s.queue.CallNext(ctx, departmentID, doctorID)
		if err != nil {
			return err
		}
		entry = e
		return nil
	})
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}

	s.audit.Record(ctx, actor.UserID, actor.Role, "queue.patient_called", "queue_entry", entry.ID, map[string]any{
		"department_id": entry.DepartmentID, "appointment_id": entry.AppointmentID,
	})
	s.publishAndInvalidate(ctx, entry.DepartmentID, cache.EventPatientCalled, entry)
	return entry, nil
}

func (s *QueueService) StartConsultation(ctx context.Context, actor Actor, queueEntryID string) (*domain.QueueEntry, error) {
	var entry *domain.QueueEntry
	err := s.pool.WithTx(ctx, func(ctx context.Context) error {
		e, err := s.queue.GetByID(ctx, queueEntryID)
		if err != nil {
			return err
		}
		if e.Status != domain.QueueCalled {
			return apperr.Conflict("only a called patient can start consultation")
		}
		if err := s.queue.UpdateStatus(ctx, e.ID, domain.QueueInProgress); err != nil {
			return err
		}

		appt, err := s.appointments.GetByID(ctx, e.AppointmentID)
		if err != nil {
			return err
		}
		if !appt.Status.CanTransition(domain.AppointmentInConsult) {
			return apperr.Conflict((&domain.InvalidTransitionError{From: appt.Status, To: domain.AppointmentInConsult}).Error())
		}
		if err := s.appointments.UpdateStatus(ctx, appt.ID, domain.AppointmentInConsult); err != nil {
			return err
		}

		e.Status = domain.QueueInProgress
		entry = e
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.audit.Record(ctx, actor.UserID, actor.Role, "queue.consultation_started", "queue_entry", entry.ID, nil)
	s.publishAndInvalidate(ctx, entry.DepartmentID, cache.EventConsultStarted, entry)
	return entry, nil
}

func (s *QueueService) CompleteConsultation(ctx context.Context, actor Actor, queueEntryID, notes string) (*domain.QueueEntry, error) {
	var entry *domain.QueueEntry
	err := s.pool.WithTx(ctx, func(ctx context.Context) error {
		e, err := s.queue.GetByID(ctx, queueEntryID)
		if err != nil {
			return err
		}
		if e.Status != domain.QueueInProgress {
			return apperr.Conflict("only a patient currently in consultation can be completed")
		}
		if err := s.queue.UpdateStatus(ctx, e.ID, domain.QueueCompleted); err != nil {
			return err
		}

		appt, err := s.appointments.GetByID(ctx, e.AppointmentID)
		if err != nil {
			return err
		}
		if !appt.Status.CanTransition(domain.AppointmentCompleted) {
			return apperr.Conflict((&domain.InvalidTransitionError{From: appt.Status, To: domain.AppointmentCompleted}).Error())
		}
		if notes != "" {
			appt.Notes = notes
			if err := s.appointments.Update(ctx, appt); err != nil {
				return err
			}
		}
		if err := s.appointments.UpdateStatus(ctx, appt.ID, domain.AppointmentCompleted); err != nil {
			return err
		}

		e.Status = domain.QueueCompleted
		entry = e
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.audit.Record(ctx, actor.UserID, actor.Role, "queue.consultation_completed", "queue_entry", entry.ID, nil)
	s.publishAndInvalidate(ctx, entry.DepartmentID, cache.EventConsultCompleted, entry)
	return entry, nil
}

// Requeue is used when a called patient does not respond; it returns them
// to the back of their priority band instead of losing their place
// entirely.
func (s *QueueService) Requeue(ctx context.Context, actor Actor, queueEntryID string) (*domain.QueueEntry, error) {
	var entry *domain.QueueEntry
	err := s.pool.WithTx(ctx, func(ctx context.Context) error {
		e, err := s.queue.GetByID(ctx, queueEntryID)
		if err != nil {
			return err
		}
		if err := s.queue.Requeue(ctx, e.ID); err != nil {
			return err
		}
		e.Status = domain.QueueWaiting
		e.CheckedInAt = time.Now()
		entry = e
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.audit.Record(ctx, actor.UserID, actor.Role, "queue.patient_requeued", "queue_entry", entry.ID, nil)
	s.publishAndInvalidate(ctx, entry.DepartmentID, cache.EventPatientRequeued, entry)
	return entry, nil
}

// MarkNoShow removes a called-but-unresponsive patient from the queue for
// good and marks their appointment as a no-show.
func (s *QueueService) MarkNoShow(ctx context.Context, actor Actor, queueEntryID string) (*domain.QueueEntry, error) {
	var entry *domain.QueueEntry
	err := s.pool.WithTx(ctx, func(ctx context.Context) error {
		e, err := s.queue.GetByID(ctx, queueEntryID)
		if err != nil {
			return err
		}
		if err := s.queue.UpdateStatus(ctx, e.ID, domain.QueueSkipped); err != nil {
			return err
		}

		appt, err := s.appointments.GetByID(ctx, e.AppointmentID)
		if err != nil {
			return err
		}
		if !appt.Status.CanTransition(domain.AppointmentNoShow) {
			return apperr.Conflict((&domain.InvalidTransitionError{From: appt.Status, To: domain.AppointmentNoShow}).Error())
		}
		if err := s.appointments.UpdateStatus(ctx, appt.ID, domain.AppointmentNoShow); err != nil {
			return err
		}

		e.Status = domain.QueueSkipped
		entry = e
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.audit.Record(ctx, actor.UserID, actor.Role, "queue.patient_no_show", "queue_entry", entry.ID, nil)
	s.publishAndInvalidate(ctx, entry.DepartmentID, cache.EventPatientNoShow, entry)
	return entry, nil
}

// Snapshot returns the current waiting line for a department, serving
// from cache when available and falling back to PostgreSQL on a cache
// miss (Redis being down degrades latency, not correctness).
func (s *QueueService) Snapshot(ctx context.Context, departmentID string) (domain.QueueSnapshot, error) {
	if cached, ok, err := s.cache.GetSnapshot(ctx, departmentID); err == nil && ok {
		return *cached, nil
	} else if err != nil {
		s.log.Warn("queue cache read failed, falling back to database", "department_id", departmentID, "error", err)
	}

	waiting, err := s.queue.ListWaiting(ctx, departmentID)
	if err != nil {
		return domain.QueueSnapshot{}, err
	}
	snapshot := domain.QueueSnapshot{DepartmentID: departmentID, Waiting: waiting, GeneratedAt: time.Now()}

	if err := s.cache.SetSnapshot(ctx, snapshot); err != nil {
		s.log.Warn("queue cache write failed", "department_id", departmentID, "error", err)
	}
	return snapshot, nil
}

func (s *QueueService) publishAndInvalidate(ctx context.Context, departmentID, eventType string, entry *domain.QueueEntry) {
	if err := s.cache.InvalidateSnapshot(ctx, departmentID); err != nil {
		s.log.Warn("failed to invalidate queue cache", "department_id", departmentID, "error", err)
	}

	waitingCount, err := s.queue.CountWaiting(ctx, departmentID)
	if err != nil {
		s.log.Warn("failed to count waiting patients", "department_id", departmentID, "error", err)
	}

	event := cache.QueueEvent{
		Type:         eventType,
		DepartmentID: departmentID,
		Entry:        entry,
		WaitingCount: waitingCount,
		OccurredAt:   time.Now(),
	}
	if err := s.cache.Publish(ctx, event); err != nil {
		s.log.Warn("failed to publish queue event", "department_id", departmentID, "error", err)
	}
}
