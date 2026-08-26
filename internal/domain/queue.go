package domain

import "time"

// QueuePriority ranks patients within a department queue. Higher values
// are served first; within the same priority, earlier check-in wins.
type QueuePriority int

const (
	PriorityNormal    QueuePriority = 0
	PriorityUrgent    QueuePriority = 1
	PriorityEmergency QueuePriority = 2
)

func (p QueuePriority) Valid() bool {
	return p >= PriorityNormal && p <= PriorityEmergency
}

func (p QueuePriority) String() string {
	switch p {
	case PriorityEmergency:
		return "emergency"
	case PriorityUrgent:
		return "urgent"
	default:
		return "normal"
	}
}

type QueueEntryStatus string

const (
	QueueWaiting    QueueEntryStatus = "waiting"
	QueueCalled     QueueEntryStatus = "called"
	QueueInProgress QueueEntryStatus = "in_progress"
	QueueCompleted  QueueEntryStatus = "completed"
	QueueSkipped    QueueEntryStatus = "skipped"
)

// QueueEntry represents one patient's position in a department queue.
// It is created at check-in and lives alongside the appointment it was
// created from.
type QueueEntry struct {
	ID            string
	AppointmentID string
	PatientID     string
	DepartmentID  string
	DoctorID      *string
	Priority      QueuePriority
	Status        QueueEntryStatus
	QueueNumber   int
	CheckedInAt   time.Time
	CalledAt      *time.Time
	StartedAt     *time.Time
	CompletedAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time

	// Populated for API responses.
	PatientName string
}

// QueueSnapshot is the point-in-time view of a department's waiting line,
// used both for REST responses and WebSocket broadcasts.
type QueueSnapshot struct {
	DepartmentID string       `json:"department_id"`
	Waiting      []QueueEntry `json:"waiting"`
	GeneratedAt  time.Time    `json:"generated_at"`
}
