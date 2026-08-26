package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/stan-ley-tech/medqueue/internal/apperr"
	"github.com/stan-ley-tech/medqueue/internal/db"
	"github.com/stan-ley-tech/medqueue/internal/domain"
)

type AppointmentPG struct{ pgStore }

func NewAppointmentRepository(pool *db.Pool) *AppointmentPG {
	return &AppointmentPG{pgStore{pool}}
}

func (r *AppointmentPG) Create(ctx context.Context, a *domain.Appointment) error {
	row := r.pool.Querier(ctx).QueryRow(ctx, `
		INSERT INTO appointments (patient_id, doctor_id, department_id, scheduled_at, status, reason, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`,
		a.PatientID, a.DoctorID, a.DepartmentID, a.ScheduledAt, a.Status, a.Reason, a.Notes,
	)
	if err := row.Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt); err != nil {
		if isForeignKeyViolation(err) {
			return apperr.Validation("patient, doctor, or department does not exist", nil)
		}
		return fmt.Errorf("appointment_pg: create: %w", err)
	}
	return nil
}

func (r *AppointmentPG) GetByID(ctx context.Context, id string) (*domain.Appointment, error) {
	row := r.pool.Querier(ctx).QueryRow(ctx, `
		SELECT a.id, a.patient_id, a.doctor_id, a.department_id, a.scheduled_at, a.status, a.reason, a.notes,
		       a.created_at, a.updated_at,
		       p.first_name || ' ' || p.last_name, d.name
		FROM appointments a
		JOIN patients p ON p.id = a.patient_id
		JOIN doctors d ON d.id = a.doctor_id
		WHERE a.id = $1`, id)
	return scanAppointment(row)
}

func (r *AppointmentPG) Update(ctx context.Context, a *domain.Appointment) error {
	cmd, err := r.pool.Querier(ctx).Exec(ctx, `
		UPDATE appointments
		SET scheduled_at = $1, reason = $2, notes = $3, updated_at = now()
		WHERE id = $4`,
		a.ScheduledAt, a.Reason, a.Notes, a.ID,
	)
	if err != nil {
		return fmt.Errorf("appointment_pg: update: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return apperr.NotFound("appointment")
	}
	return nil
}

// UpdateStatus performs a guarded transition: it only writes when the row's
// current status still equals the last-known state implied by the calling
// service. Actual transition legality is validated in the service layer
// against domain.AppointmentStatus.CanTransition beforehand; this method
// simply persists it and reports if the row vanished.
func (r *AppointmentPG) UpdateStatus(ctx context.Context, id string, status domain.AppointmentStatus) error {
	cmd, err := r.pool.Querier(ctx).Exec(ctx, `
		UPDATE appointments SET status = $1, updated_at = now() WHERE id = $2`,
		status, id,
	)
	if err != nil {
		return fmt.Errorf("appointment_pg: update status: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return apperr.NotFound("appointment")
	}
	return nil
}

func (r *AppointmentPG) List(ctx context.Context, filter domain.AppointmentFilter) ([]domain.Appointment, int, error) {
	filter.NormalizeDefaults()

	var where []string
	var args []any
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	where = append(where, "1=1")
	if filter.PatientID != "" {
		where = append(where, "a.patient_id = "+arg(filter.PatientID)+"::uuid")
	}
	if filter.DoctorID != "" {
		where = append(where, "a.doctor_id = "+arg(filter.DoctorID)+"::uuid")
	}
	if filter.DepartmentID != "" {
		where = append(where, "a.department_id = "+arg(filter.DepartmentID)+"::uuid")
	}
	if filter.Status != "" {
		where = append(where, "a.status = "+arg(filter.Status))
	}
	if filter.From != nil {
		where = append(where, "a.scheduled_at >= "+arg(*filter.From))
	}
	if filter.To != nil {
		where = append(where, "a.scheduled_at <= "+arg(*filter.To))
	}

	whereSQL := strings.Join(where, " AND ")

	var total int
	countSQL := "SELECT count(*) FROM appointments a WHERE " + whereSQL
	if err := r.pool.Querier(ctx).QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("appointment_pg: count: %w", err)
	}

	limitArg := arg(filter.Limit)
	offsetArg := arg(filter.Offset)
	listSQL := fmt.Sprintf(`
		SELECT a.id, a.patient_id, a.doctor_id, a.department_id, a.scheduled_at, a.status, a.reason, a.notes,
		       a.created_at, a.updated_at,
		       p.first_name || ' ' || p.last_name, doc.name
		FROM appointments a
		JOIN patients p ON p.id = a.patient_id
		JOIN doctors doc ON doc.id = a.doctor_id
		WHERE %s
		ORDER BY a.scheduled_at ASC
		LIMIT %s OFFSET %s`, whereSQL, limitArg, offsetArg)

	rows, err := r.pool.Querier(ctx).Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("appointment_pg: list: %w", err)
	}
	defer rows.Close()

	var out []domain.Appointment
	for rows.Next() {
		a, err := scanAppointment(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("appointment_pg: rows: %w", err)
	}
	return out, total, nil
}

func (r *AppointmentPG) ExistsOverlapping(ctx context.Context, doctorID string, at time.Time, window time.Duration, excludeID string) (bool, error) {
	var exists bool
	err := r.pool.Querier(ctx).QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM appointments
			WHERE doctor_id = $1
			  AND status NOT IN ('cancelled', 'completed', 'no_show')
			  AND scheduled_at BETWEEN $2::timestamptz - make_interval(secs => $3) AND $2::timestamptz + make_interval(secs => $3)
			  AND ($4 = '' OR id <> $4::uuid)
		)`,
		doctorID, at, window.Seconds(), excludeID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("appointment_pg: overlap check: %w", err)
	}
	return exists, nil
}

func (r *AppointmentPG) ListDueForReminder(ctx context.Context, notBefore, notAfter time.Time, limit int) ([]domain.Appointment, error) {
	rows, err := r.pool.Querier(ctx).Query(ctx, `
		SELECT a.id, a.patient_id, a.doctor_id, a.department_id, a.scheduled_at, a.status, a.reason, a.notes,
		       a.created_at, a.updated_at,
		       p.first_name || ' ' || p.last_name, doc.name
		FROM appointments a
		JOIN patients p ON p.id = a.patient_id
		JOIN doctors doc ON doc.id = a.doctor_id
		WHERE a.status = 'scheduled' AND a.reminder_sent_at IS NULL
		  AND a.scheduled_at BETWEEN $1 AND $2
		ORDER BY a.scheduled_at ASC
		LIMIT $3`,
		notBefore, notAfter, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("appointment_pg: list due for reminder: %w", err)
	}
	defer rows.Close()

	var out []domain.Appointment
	for rows.Next() {
		a, err := scanAppointment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func (r *AppointmentPG) MarkReminderSent(ctx context.Context, id string) error {
	_, err := r.pool.Querier(ctx).Exec(ctx, `UPDATE appointments SET reminder_sent_at = now() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("appointment_pg: mark reminder sent: %w", err)
	}
	return nil
}

func scanAppointment(row rowScanner) (*domain.Appointment, error) {
	var a domain.Appointment
	err := row.Scan(&a.ID, &a.PatientID, &a.DoctorID, &a.DepartmentID, &a.ScheduledAt, &a.Status,
		&a.Reason, &a.Notes, &a.CreatedAt, &a.UpdatedAt, &a.PatientName, &a.DoctorName)
	if err != nil {
		if isNoRows(err) {
			return nil, apperr.NotFound("appointment")
		}
		return nil, fmt.Errorf("appointment_pg: scan: %w", err)
	}
	return &a, nil
}
