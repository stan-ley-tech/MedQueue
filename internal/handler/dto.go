package handler

import (
	"time"

	"github.com/stan-ley-tech/medqueue/internal/domain"
	"github.com/stan-ley-tech/medqueue/internal/service"
)

// Request DTOs are decoded from the request body and validated via struct
// tags before ever reaching a service. Response DTOs shape what the
// domain model exposes over the wire, keeping persistence concerns
// (e.g. internal-only fields) out of the API contract.

type createDepartmentRequest struct {
	Name        string `json:"name" validate:"required,min=2,max=120"`
	Description string `json:"description" validate:"max=1000"`
}

type updateDepartmentRequest struct {
	Name        string `json:"name" validate:"required,min=2,max=120"`
	Description string `json:"description" validate:"max=1000"`
	Active      bool   `json:"active"`
}

type departmentResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func toDepartmentResponse(d domain.Department) departmentResponse {
	return departmentResponse{
		ID: d.ID, Name: d.Name, Description: d.Description, Active: d.Active,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

type createDoctorRequest struct {
	Name         string `json:"name" validate:"required,min=2,max=200"`
	Specialty    string `json:"specialty" validate:"max=200"`
	DepartmentID string `json:"department_id" validate:"required,uuid"`
	UserID       string `json:"user_id" validate:"omitempty,uuid"`
}

type updateDoctorRequest struct {
	Name         string `json:"name" validate:"required,min=2,max=200"`
	Specialty    string `json:"specialty" validate:"max=200"`
	DepartmentID string `json:"department_id" validate:"required,uuid"`
	Active       bool   `json:"active"`
}

type doctorResponse struct {
	ID           string    `json:"id"`
	UserID       *string   `json:"user_id,omitempty"`
	Name         string    `json:"name"`
	Specialty    string    `json:"specialty"`
	DepartmentID string    `json:"department_id"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func toDoctorResponse(d domain.Doctor) doctorResponse {
	return doctorResponse{
		ID: d.ID, UserID: d.UserID, Name: d.Name, Specialty: d.Specialty,
		DepartmentID: d.DepartmentID, Active: d.Active, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

type createPatientRequest struct {
	MedicalRecordNumber string `json:"medical_record_number" validate:"required,min=2,max=64"`
	FirstName           string `json:"first_name" validate:"required,min=1,max=120"`
	LastName            string `json:"last_name" validate:"required,min=1,max=120"`
	DateOfBirth         string `json:"date_of_birth" validate:"required,datetime=2006-01-02"`
	Sex                 string `json:"sex" validate:"omitempty,oneof=male female other unspecified"`
	Phone               string `json:"phone" validate:"omitempty,max=32"`
	Email               string `json:"email" validate:"omitempty,email"`
	Address             string `json:"address" validate:"max=500"`
}

type updatePatientRequest struct {
	FirstName   string `json:"first_name" validate:"required,min=1,max=120"`
	LastName    string `json:"last_name" validate:"required,min=1,max=120"`
	DateOfBirth string `json:"date_of_birth" validate:"required,datetime=2006-01-02"`
	Sex         string `json:"sex" validate:"omitempty,oneof=male female other unspecified"`
	Phone       string `json:"phone" validate:"omitempty,max=32"`
	Email       string `json:"email" validate:"omitempty,email"`
	Address     string `json:"address" validate:"max=500"`
}

type patientResponse struct {
	ID                  string    `json:"id"`
	MedicalRecordNumber string    `json:"medical_record_number"`
	FirstName           string    `json:"first_name"`
	LastName            string    `json:"last_name"`
	DateOfBirth         string    `json:"date_of_birth"`
	Sex                 string    `json:"sex"`
	Phone               string    `json:"phone"`
	Email               string    `json:"email"`
	Address             string    `json:"address"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func toPatientResponse(p domain.Patient) patientResponse {
	return patientResponse{
		ID: p.ID, MedicalRecordNumber: p.MedicalRecordNumber, FirstName: p.FirstName, LastName: p.LastName,
		DateOfBirth: p.DateOfBirth.Format("2006-01-02"), Sex: p.Sex, Phone: p.Phone, Email: p.Email,
		Address: p.Address, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

type scheduleAppointmentRequest struct {
	PatientID    string `json:"patient_id" validate:"required,uuid"`
	DoctorID     string `json:"doctor_id" validate:"required,uuid"`
	DepartmentID string `json:"department_id" validate:"required,uuid"`
	ScheduledAt  string `json:"scheduled_at" validate:"required"`
	Reason       string `json:"reason" validate:"max=500"`
}

type rescheduleAppointmentRequest struct {
	ScheduledAt string `json:"scheduled_at" validate:"required"`
}

type appointmentResponse struct {
	ID           string    `json:"id"`
	PatientID    string    `json:"patient_id"`
	PatientName  string    `json:"patient_name,omitempty"`
	DoctorID     string    `json:"doctor_id"`
	DoctorName   string    `json:"doctor_name,omitempty"`
	DepartmentID string    `json:"department_id"`
	ScheduledAt  time.Time `json:"scheduled_at"`
	Status       string    `json:"status"`
	Reason       string    `json:"reason"`
	Notes        string    `json:"notes,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func toAppointmentResponse(a domain.Appointment) appointmentResponse {
	return appointmentResponse{
		ID: a.ID, PatientID: a.PatientID, PatientName: a.PatientName, DoctorID: a.DoctorID, DoctorName: a.DoctorName,
		DepartmentID: a.DepartmentID, ScheduledAt: a.ScheduledAt, Status: string(a.Status), Reason: a.Reason,
		Notes: a.Notes, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}

type checkInRequest struct {
	Priority *int `json:"priority" validate:"omitempty,gte=0,lte=2"`
}

type completeConsultationRequest struct {
	Notes string `json:"notes" validate:"max=4000"`
}

type queueEntryResponse struct {
	ID            string     `json:"id"`
	AppointmentID string     `json:"appointment_id"`
	PatientID     string     `json:"patient_id"`
	PatientName   string     `json:"patient_name,omitempty"`
	DepartmentID  string     `json:"department_id"`
	DoctorID      *string    `json:"doctor_id,omitempty"`
	Priority      int        `json:"priority"`
	Status        string     `json:"status"`
	QueueNumber   int        `json:"queue_number"`
	CheckedInAt   time.Time  `json:"checked_in_at"`
	CalledAt      *time.Time `json:"called_at,omitempty"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}

func toQueueEntryResponse(q domain.QueueEntry) queueEntryResponse {
	return queueEntryResponse{
		ID: q.ID, AppointmentID: q.AppointmentID, PatientID: q.PatientID, PatientName: q.PatientName,
		DepartmentID: q.DepartmentID, DoctorID: q.DoctorID, Priority: int(q.Priority), Status: string(q.Status),
		QueueNumber: q.QueueNumber, CheckedInAt: q.CheckedInAt, CalledAt: q.CalledAt, StartedAt: q.StartedAt,
		CompletedAt: q.CompletedAt,
	}
}

type queueSnapshotResponse struct {
	DepartmentID string               `json:"department_id"`
	Waiting      []queueEntryResponse `json:"waiting"`
	GeneratedAt  time.Time            `json:"generated_at"`
}

func toQueueSnapshotResponse(s domain.QueueSnapshot) queueSnapshotResponse {
	waiting := make([]queueEntryResponse, 0, len(s.Waiting))
	for _, e := range s.Waiting {
		waiting = append(waiting, toQueueEntryResponse(e))
	}
	return queueSnapshotResponse{DepartmentID: s.DepartmentID, Waiting: waiting, GeneratedAt: s.GeneratedAt}
}

type registerRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=72"`
	Name     string `json:"name" validate:"required,min=1,max=200"`
	Role     string `json:"role" validate:"required,oneof=admin front_desk clinician"`
}

type loginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type sessionResponse struct {
	AccessToken           string       `json:"access_token"`
	AccessTokenExpiresAt  time.Time    `json:"access_token_expires_at"`
	RefreshToken          string       `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time    `json:"refresh_token_expires_at"`
	User                  userResponse `json:"user"`
}

type userResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

func toUserResponse(u domain.User) userResponse {
	return userResponse{ID: u.ID, Email: u.Email, Name: u.Name, Role: string(u.Role)}
}

func toSessionResponse(s *service.Session) sessionResponse {
	return sessionResponse{
		AccessToken: s.AccessToken, AccessTokenExpiresAt: s.AccessTokenExpiresAt,
		RefreshToken: s.RefreshToken, RefreshTokenExpiresAt: s.RefreshTokenExpiresAt,
		User: toUserResponse(*s.User),
	}
}

type pagedResponse[T any] struct {
	Items  []T `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}
