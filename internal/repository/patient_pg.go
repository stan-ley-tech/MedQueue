package repository

import (
	"context"
	"fmt"

	"github.com/stan-ley-tech/medqueue/internal/apperr"
	"github.com/stan-ley-tech/medqueue/internal/db"
	"github.com/stan-ley-tech/medqueue/internal/domain"
)

type PatientPG struct{ pgStore }

func NewPatientRepository(pool *db.Pool) *PatientPG {
	return &PatientPG{pgStore{pool}}
}

func (r *PatientPG) Create(ctx context.Context, p *domain.Patient) error {
	row := r.pool.Querier(ctx).QueryRow(ctx, `
		INSERT INTO patients (medical_record_number, first_name, last_name, date_of_birth, sex, phone, email, address)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8)
		RETURNING id, created_at, updated_at`,
		p.MedicalRecordNumber, p.FirstName, p.LastName, p.DateOfBirth, p.Sex, p.Phone, p.Email, p.Address,
	)
	if err := row.Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if isUniqueViolation(err) {
			return apperr.Conflict("a patient with this medical record number already exists")
		}
		return fmt.Errorf("patient_pg: create: %w", err)
	}
	return nil
}

func (r *PatientPG) GetByID(ctx context.Context, id string) (*domain.Patient, error) {
	row := r.pool.Querier(ctx).QueryRow(ctx, `
		SELECT id, medical_record_number, first_name, last_name, date_of_birth, sex, phone, coalesce(email, ''), address, created_at, updated_at
		FROM patients WHERE id = $1`, id)
	return scanPatient(row)
}

func (r *PatientPG) Update(ctx context.Context, p *domain.Patient) error {
	cmd, err := r.pool.Querier(ctx).Exec(ctx, `
		UPDATE patients
		SET first_name = $1, last_name = $2, date_of_birth = $3, sex = $4,
		    phone = $5, email = NULLIF($6, ''), address = $7, updated_at = now()
		WHERE id = $8`,
		p.FirstName, p.LastName, p.DateOfBirth, p.Sex, p.Phone, p.Email, p.Address, p.ID,
	)
	if err != nil {
		return fmt.Errorf("patient_pg: update: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return apperr.NotFound("patient")
	}
	return nil
}

func (r *PatientPG) List(ctx context.Context, filter domain.PatientFilter) ([]domain.Patient, int, error) {
	filter.NormalizeDefaults()

	const base = `FROM patients
		WHERE ($1 = '' OR first_name ILIKE '%' || $1 || '%' OR last_name ILIKE '%' || $1 || '%'
		       OR medical_record_number ILIKE '%' || $1 || '%' OR phone ILIKE '%' || $1 || '%')`

	var total int
	if err := r.pool.Querier(ctx).QueryRow(ctx, `SELECT count(*) `+base, filter.Search).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("patient_pg: count: %w", err)
	}

	rows, err := r.pool.Querier(ctx).Query(ctx, `
		SELECT id, medical_record_number, first_name, last_name, date_of_birth, sex, phone, coalesce(email, ''), address, created_at, updated_at `+base+`
		ORDER BY last_name ASC, first_name ASC LIMIT $2 OFFSET $3`,
		filter.Search, filter.Limit, filter.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("patient_pg: list: %w", err)
	}
	defer rows.Close()

	var out []domain.Patient
	for rows.Next() {
		p, err := scanPatient(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("patient_pg: rows: %w", err)
	}
	return out, total, nil
}

func scanPatient(row rowScanner) (*domain.Patient, error) {
	var p domain.Patient
	err := row.Scan(&p.ID, &p.MedicalRecordNumber, &p.FirstName, &p.LastName, &p.DateOfBirth,
		&p.Sex, &p.Phone, &p.Email, &p.Address, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if isNoRows(err) {
			return nil, apperr.NotFound("patient")
		}
		return nil, fmt.Errorf("patient_pg: scan: %w", err)
	}
	return &p, nil
}
