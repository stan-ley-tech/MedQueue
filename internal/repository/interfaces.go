// Package repository defines the persistence ports used by the service
// layer and implements them against PostgreSQL. Services depend only on
// these interfaces, which keeps business logic testable behind fakes and
// keeps SQL out of the service package entirely.
package repository

import (
	"context"
	"time"

	"github.com/stan-ley-tech/medqueue/internal/domain"
)

type UserRepository interface {
	Create(ctx context.Context, u *domain.User) error
	GetByID(ctx context.Context, id string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
}

type PatientRepository interface {
	Create(ctx context.Context, p *domain.Patient) error
	GetByID(ctx context.Context, id string) (*domain.Patient, error)
	Update(ctx context.Context, p *domain.Patient) error
	List(ctx context.Context, filter domain.PatientFilter) ([]domain.Patient, int, error)
}

type DoctorRepository interface {
	Create(ctx context.Context, d *domain.Doctor) error
	GetByID(ctx context.Context, id string) (*domain.Doctor, error)
	GetByUserID(ctx context.Context, userID string) (*domain.Doctor, error)
	Update(ctx context.Context, d *domain.Doctor) error
	List(ctx context.Context, filter domain.DoctorFilter) ([]domain.Doctor, int, error)
}

type DepartmentRepository interface {
	Create(ctx context.Context, d *domain.Department) error
	GetByID(ctx context.Context, id string) (*domain.Department, error)
	Update(ctx context.Context, d *domain.Department) error
	List(ctx context.Context, filter domain.DepartmentFilter) ([]domain.Department, int, error)
}

type AppointmentRepository interface {
	Create(ctx context.Context, a *domain.Appointment) error
	GetByID(ctx context.Context, id string) (*domain.Appointment, error)
	Update(ctx context.Context, a *domain.Appointment) error
	UpdateStatus(ctx context.Context, id string, status domain.AppointmentStatus) error
	List(ctx context.Context, filter domain.AppointmentFilter) ([]domain.Appointment, int, error)
	// ExistsOverlapping reports whether the doctor already has an active
	// (non-cancelled, non-completed) appointment within the exclusion
	// window around at. Used to reject double-booking.
	ExistsOverlapping(ctx context.Context, doctorID string, at time.Time, window time.Duration, excludeID string) (bool, error)
	ListDueForReminder(ctx context.Context, notBefore, notAfter time.Time, limit int) ([]domain.Appointment, error)
	MarkReminderSent(ctx context.Context, id string) error
}

type QueueRepository interface {
	// Create inserts a waiting queue entry for an appointment. It fails
	// with a conflict error if one already exists (unique index on
	// appointment_id), which is how double check-in is prevented.
	Create(ctx context.Context, q *domain.QueueEntry) error
	GetByID(ctx context.Context, id string) (*domain.QueueEntry, error)
	// CallNext atomically selects and locks the highest-priority, oldest
	// waiting entry in the department using SELECT ... FOR UPDATE SKIP
	// LOCKED, marks it called, and returns it. It returns nil, nil if the
	// queue is empty. Must be called within a transaction for the lock to
	// have effect across concurrent callers.
	CallNext(ctx context.Context, departmentID string, doctorID *string) (*domain.QueueEntry, error)
	UpdateStatus(ctx context.Context, id string, status domain.QueueEntryStatus) error
	// Requeue returns a called-but-unresponsive entry to the waiting pool
	// at the back of its priority band by bumping checked_in_at to now.
	Requeue(ctx context.Context, id string) error
	ListWaiting(ctx context.Context, departmentID string) ([]domain.QueueEntry, error)
	CountWaiting(ctx context.Context, departmentID string) (int, error)
}

type AuditRepository interface {
	Create(ctx context.Context, a *domain.AuditLog) error
	List(ctx context.Context, filter domain.AuditFilter) ([]domain.AuditLog, int, error)
}

type IdempotencyRepository interface {
	// Reserve inserts a placeholder row for key if none exists yet,
	// returning found=true and the prior stored response when the key was
	// already used. Reservation and lookup happen in a single statement
	// to avoid a race between two concurrent requests bearing the same
	// key.
	Reserve(ctx context.Context, key, path, requestHash string, ttl time.Duration) (found bool, statusCode int, responseBody []byte, err error)
	Complete(ctx context.Context, key string, statusCode int, responseBody []byte) error
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error
	GetActiveByHash(ctx context.Context, tokenHash string) (userID string, err error)
	Revoke(ctx context.Context, tokenHash string) error
	RevokeAllForUser(ctx context.Context, userID string) error
}
