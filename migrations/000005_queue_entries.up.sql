-- Drives the human-readable "now serving #42" ticket number. A single
-- global sequence is sufficient here and, unlike a per-department MAX()+1
-- query, never races under concurrent check-ins.
CREATE SEQUENCE queue_number_seq;

CREATE TABLE queue_entries (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    appointment_id UUID NOT NULL REFERENCES appointments (id) ON DELETE RESTRICT,
    patient_id     UUID NOT NULL REFERENCES patients (id) ON DELETE RESTRICT,
    department_id  UUID NOT NULL REFERENCES departments (id) ON DELETE RESTRICT,
    doctor_id      UUID REFERENCES doctors (id) ON DELETE SET NULL,
    priority       SMALLINT NOT NULL DEFAULT 0 CHECK (priority BETWEEN 0 AND 2),
    status         TEXT NOT NULL DEFAULT 'waiting'
                       CHECK (status IN ('waiting', 'called', 'in_progress', 'completed', 'skipped')),
    queue_number   INTEGER NOT NULL DEFAULT nextval('queue_number_seq'),
    checked_in_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    called_at      TIMESTAMPTZ,
    started_at     TIMESTAMPTZ,
    completed_at   TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER SEQUENCE queue_number_seq OWNED BY queue_entries.queue_number;

-- One active queue entry per appointment: prevents double check-in.
CREATE UNIQUE INDEX idx_queue_entries_appointment_id ON queue_entries (appointment_id);

-- The "call next patient" query: waiting entries in a department ordered
-- by priority then arrival time. This partial index covers it directly.
CREATE INDEX idx_queue_entries_dept_waiting ON queue_entries (department_id, priority DESC, checked_in_at ASC)
    WHERE status = 'waiting';

CREATE INDEX idx_queue_entries_doctor_active ON queue_entries (doctor_id, status)
    WHERE status IN ('called', 'in_progress');
