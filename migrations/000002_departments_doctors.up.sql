CREATE TABLE departments (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    active      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_departments_name ON departments (name);

CREATE TABLE doctors (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID REFERENCES users (id) ON DELETE SET NULL,
    department_id UUID NOT NULL REFERENCES departments (id) ON DELETE RESTRICT,
    name          TEXT NOT NULL,
    specialty     TEXT NOT NULL DEFAULT '',
    active        BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_doctors_department_id ON doctors (department_id);
CREATE UNIQUE INDEX idx_doctors_user_id ON doctors (user_id) WHERE user_id IS NOT NULL;
