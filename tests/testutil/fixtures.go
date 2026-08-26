package testutil

import (
	"context"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stan-ley-tech/medqueue/internal/auth"
	"github.com/stan-ley-tech/medqueue/internal/db"
	"github.com/stan-ley-tech/medqueue/internal/domain"
	"github.com/stan-ley-tech/medqueue/internal/repository"
)

// TruncateAll clears every application table so each test starts from a
// clean slate without needing a fresh database per test.
func TruncateAll(t *testing.T, pool *db.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		TRUNCATE TABLE
			audit_logs, idempotency_keys, refresh_tokens,
			queue_entries, appointments, doctors, patients, departments, users
		RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("testutil: truncate tables: %v", err)
	}
}

// SeedDepartment inserts a department with a randomized name so parallel
// tests never collide on the unique index.
func SeedDepartment(t *testing.T, pool *db.Pool) *domain.Department {
	t.Helper()
	d := &domain.Department{Name: "Test Department " + uniqueSuffix(), Description: "seeded for tests"}
	if err := repository.NewDepartmentRepository(pool).Create(context.Background(), d); err != nil {
		t.Fatalf("testutil: seed department: %v", err)
	}
	return d
}

func SeedDoctor(t *testing.T, pool *db.Pool, departmentID string) *domain.Doctor {
	t.Helper()
	d := &domain.Doctor{Name: "Dr. Test " + uniqueSuffix(), Specialty: "General", DepartmentID: departmentID}
	if err := repository.NewDoctorRepository(pool).Create(context.Background(), d); err != nil {
		t.Fatalf("testutil: seed doctor: %v", err)
	}
	return d
}

func SeedPatient(t *testing.T, pool *db.Pool) *domain.Patient {
	t.Helper()
	suffix := uniqueSuffix()
	p := &domain.Patient{
		MedicalRecordNumber: "MRN-" + suffix,
		FirstName:           "Test",
		LastName:            "Patient-" + suffix,
		DateOfBirth:         time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := repository.NewPatientRepository(pool).Create(context.Background(), p); err != nil {
		t.Fatalf("testutil: seed patient: %v", err)
	}
	return p
}

// SeedCheckedInQueueEntry creates a patient, an already-checked-in
// appointment, and a waiting queue entry in one call, which is the
// precondition most queue tests start from.
func SeedCheckedInQueueEntry(t *testing.T, pool *db.Pool, departmentID, doctorID string, priority domain.QueuePriority) *domain.QueueEntry {
	t.Helper()
	ctx := context.Background()

	patient := SeedPatient(t, pool)

	appt := &domain.Appointment{
		PatientID: patient.ID, DoctorID: doctorID, DepartmentID: departmentID,
		ScheduledAt: time.Now().Add(time.Hour), Status: domain.AppointmentScheduled,
	}
	apptRepo := repository.NewAppointmentRepository(pool)
	if err := apptRepo.Create(ctx, appt); err != nil {
		t.Fatalf("testutil: seed appointment: %v", err)
	}
	if err := apptRepo.UpdateStatus(ctx, appt.ID, domain.AppointmentCheckedIn); err != nil {
		t.Fatalf("testutil: transition to checked_in: %v", err)
	}
	if err := apptRepo.UpdateStatus(ctx, appt.ID, domain.AppointmentInQueue); err != nil {
		t.Fatalf("testutil: transition to in_queue: %v", err)
	}

	entry := &domain.QueueEntry{
		AppointmentID: appt.ID, PatientID: patient.ID, DepartmentID: departmentID, Priority: priority,
	}
	if err := repository.NewQueueRepository(pool).Create(ctx, entry); err != nil {
		t.Fatalf("testutil: seed queue entry: %v", err)
	}
	return entry
}

// SeedUser creates a login-ready user directly against the repository,
// bypassing the HTTP registration endpoint (which itself requires an
// authenticated admin) so e2e tests have a way to bootstrap their first
// account. Returns the plaintext password alongside the user for use in
// a subsequent /auth/login call.
func SeedUser(t *testing.T, pool *db.Pool, email, password string, role domain.Role) *domain.User {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("testutil: hash password: %v", err)
	}
	u := &domain.User{Email: email, PasswordHash: hash, Name: "Seeded " + string(role), Role: role}
	if err := repository.NewUserRepository(pool).Create(context.Background(), u); err != nil {
		t.Fatalf("testutil: seed user: %v", err)
	}
	return u
}

var suffixCounter atomic.Int64

// uniqueSuffix is safe to call concurrently, which matters for the load
// test that seeds many queue entries from parallel goroutines.
func uniqueSuffix() string {
	n := suffixCounter.Add(1)
	return strconv.FormatInt(time.Now().UnixNano(), 36) + "-" + strconv.FormatInt(n, 36)
}
