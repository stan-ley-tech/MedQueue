//go:build e2e

package e2e

import (
	"net/http"
	"testing"
	"time"

	"github.com/stan-ley-tech/medqueue/internal/domain"
	"github.com/stan-ley-tech/medqueue/tests/testutil"
)

// TestFullPatientWorkflow drives the entire clinic workflow end to end
// through real HTTP requests against a live server backed by a migrated
// PostgreSQL database and Redis: patient created, appointment scheduled,
// checked in, queued, called, consulted, and completed. This is the
// scenario every other test only covers a slice of.
func TestFullPatientWorkflow(t *testing.T) {
	pool := testutil.RequireDB(t)
	redisClient := testutil.RequireRedis(t)
	testutil.TruncateAll(t, pool)

	srv := testutil.NewTestServer(t, pool, redisClient)
	anon := newAPIClient(t, srv.URL)

	// Bootstrap: an admin and a clinician account, seeded directly since
	// no admin exists yet to call POST /auth/register.
	adminPassword := "admin-password-123"
	testutil.SeedUser(t, pool, "admin@medqueue.test", adminPassword, domain.RoleAdmin)
	frontDeskPassword := "frontdesk-password-123"
	testutil.SeedUser(t, pool, "frontdesk@medqueue.test", frontDeskPassword, domain.RoleFrontDesk)

	admin := loginAs(t, anon, "admin@medqueue.test", adminPassword)
	frontDesk := loginAs(t, anon, "frontdesk@medqueue.test", frontDeskPassword)

	// Admin creates a department and a doctor.
	var department struct {
		ID string `json:"id"`
	}
	resp, body := admin.do(http.MethodPost, "/api/v1/departments", map[string]any{
		"name": "Cardiology " + time.Now().Format("150405.000000"),
	}, &department)
	admin.requireStatus(resp, body, http.StatusCreated)

	doctorEmail := "clinician@medqueue.test"
	doctorPassword := "clinician-password-123"
	doctorUser := testutil.SeedUser(t, pool, doctorEmail, doctorPassword, domain.RoleClinician)

	var doctor struct {
		ID string `json:"id"`
	}
	resp, body = admin.do(http.MethodPost, "/api/v1/doctors", map[string]any{
		"name":          "Dr. House",
		"specialty":     "Cardiology",
		"department_id": department.ID,
		"user_id":       doctorUser.ID,
	}, &doctor)
	admin.requireStatus(resp, body, http.StatusCreated)

	clinician := loginAs(t, anon, doctorEmail, doctorPassword)

	// Front desk registers a patient.
	var patient struct {
		ID string `json:"id"`
	}
	resp, body = frontDesk.do(http.MethodPost, "/api/v1/patients", map[string]any{
		"medical_record_number": "MRN-E2E-" + time.Now().Format("150405.000000"),
		"first_name":            "Grace",
		"last_name":             "Hopper",
		"date_of_birth":         "1985-12-09",
	}, &patient)
	frontDesk.requireStatus(resp, body, http.StatusCreated)

	// Front desk schedules an appointment.
	var appointment struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	resp, body = frontDesk.do(http.MethodPost, "/api/v1/appointments", map[string]any{
		"patient_id":    patient.ID,
		"doctor_id":     doctor.ID,
		"department_id": department.ID,
		"scheduled_at":  time.Now().Add(time.Hour).Format(time.RFC3339),
		"reason":        "annual checkup",
	}, &appointment)
	frontDesk.requireStatus(resp, body, http.StatusCreated)
	if appointment.Status != string(domain.AppointmentScheduled) {
		t.Fatalf("expected status scheduled, got %s", appointment.Status)
	}

	// Front desk checks the patient in, which enqueues them.
	var queueEntry struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	resp, body = frontDesk.do(http.MethodPost, "/api/v1/appointments/"+appointment.ID+"/check-in", map[string]any{
		"priority": 1,
	}, &queueEntry)
	frontDesk.requireStatus(resp, body, http.StatusCreated)
	if queueEntry.Status != string(domain.QueueWaiting) {
		t.Fatalf("expected queue entry status waiting, got %s", queueEntry.Status)
	}

	// Department queue snapshot should show one waiting patient.
	var snapshot struct {
		Waiting []map[string]any `json:"waiting"`
	}
	resp, body = frontDesk.do(http.MethodGet, "/api/v1/departments/"+department.ID+"/queue", nil, &snapshot)
	frontDesk.requireStatus(resp, body, http.StatusOK)
	if len(snapshot.Waiting) != 1 {
		t.Fatalf("expected 1 waiting patient in snapshot, got %d", len(snapshot.Waiting))
	}

	// Clinician calls the next patient.
	var called struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	resp, body = clinician.do(http.MethodPost, "/api/v1/departments/"+department.ID+"/queue/call-next", nil, &called)
	clinician.requireStatus(resp, body, http.StatusOK)
	if called.ID != queueEntry.ID {
		t.Fatalf("expected call-next to claim entry %s, got %s", queueEntry.ID, called.ID)
	}
	if called.Status != string(domain.QueueCalled) {
		t.Fatalf("expected status called, got %s", called.Status)
	}

	// Clinician starts the consultation.
	var started struct {
		Status string `json:"status"`
	}
	resp, body = clinician.do(http.MethodPost, "/api/v1/queue/"+queueEntry.ID+"/start", nil, &started)
	clinician.requireStatus(resp, body, http.StatusOK)
	if started.Status != string(domain.QueueInProgress) {
		t.Fatalf("expected status in_progress, got %s", started.Status)
	}

	// Appointment should now be in_consultation.
	var apptAfterStart struct {
		Status string `json:"status"`
	}
	resp, body = clinician.do(http.MethodGet, "/api/v1/appointments/"+appointment.ID, nil, &apptAfterStart)
	clinician.requireStatus(resp, body, http.StatusOK)
	if apptAfterStart.Status != string(domain.AppointmentInConsult) {
		t.Fatalf("expected appointment status in_consultation, got %s", apptAfterStart.Status)
	}

	// Clinician completes the consultation.
	var completed struct {
		Status string `json:"status"`
	}
	resp, body = clinician.do(http.MethodPost, "/api/v1/queue/"+queueEntry.ID+"/complete", map[string]any{
		"notes": "patient is healthy",
	}, &completed)
	clinician.requireStatus(resp, body, http.StatusOK)
	if completed.Status != string(domain.QueueCompleted) {
		t.Fatalf("expected status completed, got %s", completed.Status)
	}

	// Appointment should now be completed too.
	var apptFinal struct {
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}
	resp, body = clinician.do(http.MethodGet, "/api/v1/appointments/"+appointment.ID, nil, &apptFinal)
	clinician.requireStatus(resp, body, http.StatusOK)
	if apptFinal.Status != string(domain.AppointmentCompleted) {
		t.Fatalf("expected appointment status completed, got %s", apptFinal.Status)
	}
	if apptFinal.Notes != "patient is healthy" {
		t.Fatalf("expected consultation notes to be persisted, got %q", apptFinal.Notes)
	}

	// Queue should be empty again.
	resp, body = frontDesk.do(http.MethodGet, "/api/v1/departments/"+department.ID+"/queue", nil, &snapshot)
	frontDesk.requireStatus(resp, body, http.StatusOK)
	if len(snapshot.Waiting) != 0 {
		t.Fatalf("expected empty queue after completion, got %d waiting", len(snapshot.Waiting))
	}
}

// TestIdempotentAppointmentBooking checks that retrying a schedule
// request with the same Idempotency-Key returns the original appointment
// instead of creating a duplicate booking.
func TestIdempotentAppointmentBooking(t *testing.T) {
	pool := testutil.RequireDB(t)
	redisClient := testutil.RequireRedis(t)
	testutil.TruncateAll(t, pool)

	srv := testutil.NewTestServer(t, pool, redisClient)
	anon := newAPIClient(t, srv.URL)

	adminPassword := "admin-password-123"
	testutil.SeedUser(t, pool, "admin2@medqueue.test", adminPassword, domain.RoleAdmin)
	admin := loginAs(t, anon, "admin2@medqueue.test", adminPassword)

	var department struct {
		ID string `json:"id"`
	}
	resp, body := admin.do(http.MethodPost, "/api/v1/departments", map[string]any{"name": "ER " + time.Now().Format("150405.000000")}, &department)
	admin.requireStatus(resp, body, http.StatusCreated)

	var doctor struct {
		ID string `json:"id"`
	}
	resp, body = admin.do(http.MethodPost, "/api/v1/doctors", map[string]any{
		"name": "Dr. Idempotent", "department_id": department.ID,
	}, &doctor)
	admin.requireStatus(resp, body, http.StatusCreated)

	var patient struct {
		ID string `json:"id"`
	}
	resp, body = admin.do(http.MethodPost, "/api/v1/patients", map[string]any{
		"medical_record_number": "MRN-IDEMP-" + time.Now().Format("150405.000000"),
		"first_name":            "Idem",
		"last_name":             "Potent",
		"date_of_birth":         "2000-01-01",
	}, &patient)
	admin.requireStatus(resp, body, http.StatusCreated)

	req := map[string]any{
		"patient_id": patient.ID, "doctor_id": doctor.ID, "department_id": department.ID,
		"scheduled_at": time.Now().Add(2 * time.Hour).Format(time.RFC3339),
	}

	client := &idempotentClient{apiClient: admin, key: "fixed-key-abc"}
	var first struct {
		ID string `json:"id"`
	}
	resp, body = client.do(http.MethodPost, "/api/v1/appointments", req, &first)
	client.requireStatus(resp, body, http.StatusCreated)

	var second struct {
		ID string `json:"id"`
	}
	resp, body = client.do(http.MethodPost, "/api/v1/appointments", req, &second)
	client.requireStatus(resp, body, http.StatusCreated)

	if first.ID != second.ID {
		t.Fatalf("expected idempotent replay to return the same appointment ID, got %s and %s", first.ID, second.ID)
	}
}

func loginAs(t *testing.T, anon *apiClient, email, password string) *apiClient {
	t.Helper()
	var session struct {
		AccessToken string `json:"access_token"`
	}
	resp, body := anon.do(http.MethodPost, "/api/v1/auth/login", map[string]any{
		"email": email, "password": password,
	}, &session)
	anon.requireStatus(resp, body, http.StatusOK)
	return anon.withToken(session.AccessToken)
}
