package repository

import (
	"context"
	"fmt"

	"github.com/stan-ley-tech/medqueue/internal/apperr"
	"github.com/stan-ley-tech/medqueue/internal/db"
	"github.com/stan-ley-tech/medqueue/internal/domain"
)

type DepartmentPG struct{ pgStore }

func NewDepartmentRepository(pool *db.Pool) *DepartmentPG {
	return &DepartmentPG{pgStore{pool}}
}

func (r *DepartmentPG) Create(ctx context.Context, d *domain.Department) error {
	row := r.pool.Querier(ctx).QueryRow(ctx, `
		INSERT INTO departments (name, description, active)
		VALUES ($1, $2, TRUE)
		RETURNING id, created_at, updated_at`,
		d.Name, d.Description,
	)
	if err := row.Scan(&d.ID, &d.CreatedAt, &d.UpdatedAt); err != nil {
		if isUniqueViolation(err) {
			return apperr.Conflict("a department with this name already exists")
		}
		return fmt.Errorf("department_pg: create: %w", err)
	}
	d.Active = true
	return nil
}

func (r *DepartmentPG) GetByID(ctx context.Context, id string) (*domain.Department, error) {
	row := r.pool.Querier(ctx).QueryRow(ctx, `
		SELECT id, name, description, active, created_at, updated_at
		FROM departments WHERE id = $1`, id)
	return scanDepartment(row)
}

func (r *DepartmentPG) Update(ctx context.Context, d *domain.Department) error {
	cmd, err := r.pool.Querier(ctx).Exec(ctx, `
		UPDATE departments
		SET name = $1, description = $2, active = $3, updated_at = now()
		WHERE id = $4`,
		d.Name, d.Description, d.Active, d.ID,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return apperr.Conflict("a department with this name already exists")
		}
		return fmt.Errorf("department_pg: update: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return apperr.NotFound("department")
	}
	return nil
}

func (r *DepartmentPG) List(ctx context.Context, filter domain.DepartmentFilter) ([]domain.Department, int, error) {
	filter.NormalizeDefaults()

	const base = `FROM departments WHERE ($1 = '' OR name ILIKE '%' || $1 || '%')`

	var total int
	if err := r.pool.Querier(ctx).QueryRow(ctx, `SELECT count(*) `+base, filter.Search).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("department_pg: count: %w", err)
	}

	rows, err := r.pool.Querier(ctx).Query(ctx, `
		SELECT id, name, description, active, created_at, updated_at `+base+`
		ORDER BY name ASC LIMIT $2 OFFSET $3`,
		filter.Search, filter.Limit, filter.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("department_pg: list: %w", err)
	}
	defer rows.Close()

	var out []domain.Department
	for rows.Next() {
		d, err := scanDepartment(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *d)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("department_pg: rows: %w", err)
	}
	return out, total, nil
}

func scanDepartment(row rowScanner) (*domain.Department, error) {
	var d domain.Department
	err := row.Scan(&d.ID, &d.Name, &d.Description, &d.Active, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if isNoRows(err) {
			return nil, apperr.NotFound("department")
		}
		return nil, fmt.Errorf("department_pg: scan: %w", err)
	}
	return &d, nil
}
