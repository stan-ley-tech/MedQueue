// Package notifier provides Notifier implementations for the reminder
// worker. LogNotifier is the default: it writes a structured log line
// instead of calling an SMS/email provider, so the worker's scheduling
// and retry logic can be exercised without external credentials. Swapping
// in a real provider means implementing service.Notifier against
// Twilio/SendGrid/etc. and wiring it in cmd/worker/main.go; nothing else
// in the reminder pipeline needs to change.
package notifier

import (
	"context"
	"log/slog"
	"time"
)

type LogNotifier struct {
	log *slog.Logger
}

func NewLogNotifier(log *slog.Logger) *LogNotifier {
	return &LogNotifier{log: log}
}

func (n *LogNotifier) SendAppointmentReminder(ctx context.Context, patientID, appointmentID string, scheduledAt time.Time) error {
	n.log.Info("reminder dispatched",
		"patient_id", patientID,
		"appointment_id", appointmentID,
		"scheduled_at", scheduledAt.Format(time.RFC3339),
	)
	return nil
}
