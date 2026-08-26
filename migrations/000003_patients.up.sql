CREATE TABLE patients (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    medical_record_number  TEXT NOT NULL,
    first_name             TEXT NOT NULL,
    last_name              TEXT NOT NULL,
    date_of_birth          DATE NOT NULL,
    sex                    TEXT NOT NULL DEFAULT '',
    phone                  TEXT NOT NULL DEFAULT '',
    email                  CITEXT,
    address                TEXT NOT NULL DEFAULT '',
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_patients_mrn ON patients (medical_record_number);

-- Backs "search by name" without a full scan; trigram index would be
-- preferable at very large scale, but this covers prefix search cleanly.
CREATE INDEX idx_patients_last_first_name ON patients (last_name, first_name);
CREATE INDEX idx_patients_phone ON patients (phone) WHERE phone <> '';
