package repository

import (
	"context"
	"fmt"

	"github.com/stan-ley-tech/medqueue/internal/apperr"
	"github.com/stan-ley-tech/medqueue/internal/db"
	"github.com/stan-ley-tech/medqueue/internal/domain"
)

type DoctorPG struct{ pgStore }

func NewDoctorRepository(pool *db.Pool) *DoctorPG {
	return &DoctorPG{pgStore{pool}}
}

func (r *DoctorPG) Create(ctx context.Context, d *domain.Doctor) error {
	row := r.pool.Querier(ctx).QueryRow(ctx, `
		INSERT INTO doctors (user_id, department_id, name, specialty, active)
		VALUES ($1, $2, $3, $4, TRUE)
		RETURNING id, created_at, updated_at`,
		d.UserID, d.DepartmentID, d.Name, d.Specialty,
	)
	if err := row.Scan(&d.ID, &d.CreatedAt, &d.UpdatedAt); err != nil {
		if isForeignKeyViolation(err) {
			return apperr.Validation("department does not exist", map[string]string{"department_id": "unknown department"})
		}
		if isUniqueViolation(err) {
			return apperr.Conflict("this user is already linked to a doctor profile")
		}
		return fmt.Errorf("doctor_pg: create: %w", err)
	}
	d.Active = true
	return nil
}

func (r *DoctorPG) GetByID(ctx context.Context, id string) (*domain.Doctor, error) {
	row := r.pool.Querier(ctx).QueryRow(ctx, `
		SELECT id, user_id, department_id, name, specialty, active, created_at, updated_at
		FROM doctors WHERE id = $1`, id)
	return scanDoctor(row)
}

func (r *DoctorPG) GetByUserID(ctx context.Context, userID string) (*domain.Doctor, error) {
	row := r.pool.Querier(ctx).QueryRow(ctx, `
		SELECT id, user_id, department_id, name, specialty, active, created_at, updated_at
		FROM doctors WHERE user_id = $1`, userID)
	return scanDoctor(row)
}

func (r *DoctorPG) Update(ctx context.Context, d *domain.Doctor) error {
	cmd, err := r.pool.Querier(ctx).Exec(ctx, `
		UPDATE doctors
		SET department_id = $1, name = $2, specialty = $3, active = $4, updated_at = now()
		WHERE id = $5`,
		d.DepartmentID, d.Name, d.Specialty, d.Active, d.ID,
	)
	if err != nil {
		return fmt.Errorf("doctor_pg: update: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return apperr.NotFound("doctor")
	}
	return nil
}

func (r *DoctorPG) List(ctx context.Context, filter domain.DoctorFilter) ([]domain.Doctor, int, error) {
	filter.NormalizeDefaults()

	const base = `FROM doctors
		WHERE ($1 = '' OR department_id = $1::uuid)
		AND ($2::bool IS NULL OR active = $2)`

	var activeArg any
	if filter.Active != nil {
		activeArg = *filter.Active
	}

	var total int
	if err := r.pool.Querier(ctx).QueryRow(ctx, `SELECT count(*) `+base, filter.DepartmentID, activeArg).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("doctor_pg: count: %w", err)
	}

	rows, err := r.pool.Querier(ctx).Query(ctx, `
		SELECT id, user_id, department_id, name, specialty, active, created_at, updated_at `+base+`
		ORDER BY name ASC LIMIT $3 OFFSET $4`,
		filter.DepartmentID, activeArg, filter.Limit, filter.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("doctor_pg: list: %w", err)
	}
	defer rows.Close()

	var out []domain.Doctor
	for rows.Next() {
		d, err := scanDoctor(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *d)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("doctor_pg: rows: %w", err)
	}
	return out, total, nil
}

func scanDoctor(row rowScanner) (*domain.Doctor, error) {
	var d domain.Doctor
	err := row.Scan(&d.ID, &d.UserID, &d.DepartmentID, &d.Name, &d.Specialty, &d.Active, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if isNoRows(err) {
			return nil, apperr.NotFound("doctor")
		}
		return nil, fmt.Errorf("doctor_pg: scan: %w", err)
	}
	return &d, nil
}
