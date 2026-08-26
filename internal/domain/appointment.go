package domain

import (
	"fmt"
	"time"
)

// AppointmentStatus models the lifecycle of a single appointment. Valid
// transitions are enforced by CanTransition so the service layer and the
// database constraint stay in agreement.
type AppointmentStatus string

const (
	AppointmentScheduled    AppointmentStatus = "scheduled"
	AppointmentCheckedIn    AppointmentStatus = "checked_in"
	AppointmentInQueue      AppointmentStatus = "in_queue"
	AppointmentInConsult    AppointmentStatus = "in_consultation"
	AppointmentCompleted    AppointmentStatus = "completed"
	AppointmentCancelled    AppointmentStatus = "cancelled"
	AppointmentNoShow       AppointmentStatus = "no_show"
)

// validTransitions enumerates the directed edges of the appointment state
// machine. Anything not listed here is rejected.
var validTransitions = map[AppointmentStatus][]AppointmentStatus{
	AppointmentScheduled: {AppointmentCheckedIn, AppointmentCancelled, AppointmentNoShow},
	AppointmentCheckedIn: {AppointmentInQueue, AppointmentCancelled},
	AppointmentInQueue:   {AppointmentInConsult, AppointmentCancelled},
	AppointmentInConsult: {AppointmentCompleted},
	AppointmentCompleted: {},
	AppointmentCancelled: {},
	AppointmentNoShow:    {},
}

func (s AppointmentStatus) CanTransition(to AppointmentStatus) bool {
	for _, allowed := range validTransitions[s] {
		if allowed == to {
			return true
		}
	}
	return false
}

func (s AppointmentStatus) Valid() bool {
	_, ok := validTransitions[s]
	return ok
}

type Appointment struct {
	ID             string
	PatientID      string
	DoctorID       string
	DepartmentID   string
	ScheduledAt    time.Time
	Status         AppointmentStatus
	Reason         string
	Notes          string
	CreatedAt      time.Time
	UpdatedAt      time.Time

	// Populated on read-heavy endpoints via joins; empty on writes.
	PatientName string
	DoctorName  string
}

type AppointmentFilter struct {
	PatientID    string
	DoctorID     string
	DepartmentID string
	Status       AppointmentStatus
	From         *time.Time
	To           *time.Time
	Page
}

// InvalidTransitionError is returned by the service layer when a status
// change is not permitted from the appointment's current state.
type InvalidTransitionError struct {
	From AppointmentStatus
	To   AppointmentStatus
}

func (e *InvalidTransitionError) Error() string {
	return fmt.Sprintf("cannot transition appointment from %q to %q", e.From, e.To)
}
