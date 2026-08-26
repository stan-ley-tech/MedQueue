package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/stan-ley-tech/medqueue/internal/repository"
)

// Notifier abstracts however a reminder actually reaches a patient (SMS,
// email, push). The reminder worker doesn't need to know which; it only
// needs to know delivery was attempted.
type Notifier interface {
	SendAppointmentReminder(ctx context.Context, patientID, appointmentID string, scheduledAt time.Time) error
}

// ReminderService scans for upcoming appointments that haven't been
// reminded yet and dispatches a reminder for each. It is driven by the
// background worker on a fixed interval, not by HTTP requests.
type ReminderService struct {
	appointments repository.AppointmentRepository
	notifier     Notifier
	lookahead    time.Duration
	log          *slog.Logger
}

func NewReminderService(appointments repository.AppointmentRepository, notifier Notifier, lookahead time.Duration, log *slog.Logger) *ReminderService {
	return &ReminderService{appointments: appointments, notifier: notifier, lookahead: lookahead, log: log}
}

// RunOnce scans [now, now+lookahead] for scheduled, not-yet-reminded
// appointments and sends a reminder for each, marking it sent so the next
// scan doesn't repeat it. Failures are logged and skipped rather than
// aborting the batch, so one bad send doesn't block the rest.
func (s *ReminderService) RunOnce(ctx context.Context) (sent int, err error) {
	now := time.Now()
	due, err := s.appointments.ListDueForReminder(ctx, now, now.Add(s.lookahead), 200)
	if err != nil {
		return 0, err
	}

	for _, appt := range due {
		if err := s.notifier.SendAppointmentReminder(ctx, appt.PatientID, appt.ID, appt.ScheduledAt); err != nil {
			s.log.Error("reminder: failed to send", "appointment_id", appt.ID, "error", err)
			continue
		}
		if err := s.appointments.MarkReminderSent(ctx, appt.ID); err != nil {
			s.log.Error("reminder: failed to mark sent", "appointment_id", appt.ID, "error", err)
			continue
		}
		sent++
	}
	return sent, nil
}
