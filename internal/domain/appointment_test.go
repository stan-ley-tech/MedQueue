package domain

import "testing"

func TestAppointmentStatus_CanTransition(t *testing.T) {
	tests := []struct {
		name    string
		from    AppointmentStatus
		to      AppointmentStatus
		allowed bool
	}{
		{"scheduled to checked_in", AppointmentScheduled, AppointmentCheckedIn, true},
		{"scheduled to cancelled", AppointmentScheduled, AppointmentCancelled, true},
		{"scheduled to no_show", AppointmentScheduled, AppointmentNoShow, true},
		{"scheduled to in_consultation skips queue", AppointmentScheduled, AppointmentInConsult, false},
		{"checked_in to in_queue", AppointmentCheckedIn, AppointmentInQueue, true},
		{"checked_in to completed", AppointmentCheckedIn, AppointmentCompleted, false},
		{"in_queue to in_consultation", AppointmentInQueue, AppointmentInConsult, true},
		{"in_queue to no_show", AppointmentInQueue, AppointmentNoShow, true},
		{"in_consultation to completed", AppointmentInConsult, AppointmentCompleted, true},
		{"in_consultation to cancelled not allowed", AppointmentInConsult, AppointmentCancelled, false},
		{"completed is terminal", AppointmentCompleted, AppointmentScheduled, false},
		{"cancelled is terminal", AppointmentCancelled, AppointmentCheckedIn, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.from.CanTransition(tt.to)
			if got != tt.allowed {
				t.Errorf("CanTransition(%s -> %s) = %v, want %v", tt.from, tt.to, got, tt.allowed)
			}
		})
	}
}

func TestAppointmentStatus_Valid(t *testing.T) {
	if !AppointmentScheduled.Valid() {
		t.Error("expected scheduled to be a valid status")
	}
	if AppointmentStatus("bogus").Valid() {
		t.Error("expected bogus status to be invalid")
	}
}

func TestQueuePriority_Valid(t *testing.T) {
	if !PriorityNormal.Valid() || !PriorityUrgent.Valid() || !PriorityEmergency.Valid() {
		t.Error("expected all defined priorities to be valid")
	}
	if QueuePriority(3).Valid() {
		t.Error("expected priority 3 to be invalid")
	}
	if QueuePriority(-1).Valid() {
		t.Error("expected priority -1 to be invalid")
	}
}

func TestPage_NormalizeDefaults(t *testing.T) {
	p := Page{}
	p.NormalizeDefaults()
	if p.Limit != DefaultPageLimit {
		t.Errorf("expected default limit %d, got %d", DefaultPageLimit, p.Limit)
	}
	if p.Offset != 0 {
		t.Errorf("expected default offset 0, got %d", p.Offset)
	}

	p = Page{Limit: 500, Offset: -5}
	p.NormalizeDefaults()
	if p.Limit != MaxPageLimit {
		t.Errorf("expected limit capped at %d, got %d", MaxPageLimit, p.Limit)
	}
	if p.Offset != 0 {
		t.Errorf("expected negative offset clamped to 0, got %d", p.Offset)
	}
}
