CREATE TABLE appointments (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id     UUID NOT NULL REFERENCES patients (id) ON DELETE RESTRICT,
    doctor_id      UUID NOT NULL REFERENCES doctors (id) ON DELETE RESTRICT,
    department_id  UUID NOT NULL REFERENCES departments (id) ON DELETE RESTRICT,
    scheduled_at   TIMESTAMPTZ NOT NULL,
    status         TEXT NOT NULL DEFAULT 'scheduled'
                       CHECK (status IN ('scheduled', 'checked_in', 'in_queue', 'in_consultation', 'completed', 'cancelled', 'no_show')),
    reason         TEXT NOT NULL DEFAULT '',
    notes          TEXT NOT NULL DEFAULT '',
    reminder_sent_at TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Doctor's schedule view and the "double-booked slot" check both filter on
-- (doctor_id, scheduled_at); this index serves both.
CREATE INDEX idx_appointments_doctor_scheduled ON appointments (doctor_id, scheduled_at);
CREATE INDEX idx_appointments_patient_id ON appointments (patient_id, scheduled_at DESC);
CREATE INDEX idx_appointments_department_status ON appointments (department_id, status);

-- Reminder worker scans for upcoming, not-yet-reminded, still-scheduled
-- appointments; a partial index keeps that scan cheap as the table grows.
CREATE INDEX idx_appointments_reminder_pending ON appointments (scheduled_at)
    WHERE status = 'scheduled' AND reminder_sent_at IS NULL;
