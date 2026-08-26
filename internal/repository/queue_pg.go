package repository

import (
	"context"
	"fmt"

	"github.com/stan-ley-tech/medqueue/internal/apperr"
	"github.com/stan-ley-tech/medqueue/internal/db"
	"github.com/stan-ley-tech/medqueue/internal/domain"
)

type QueuePG struct{ pgStore }

func NewQueueRepository(pool *db.Pool) *QueuePG {
	return &QueuePG{pgStore{pool}}
}

func (r *QueuePG) Create(ctx context.Context, q *domain.QueueEntry) error {
	row := r.pool.Querier(ctx).QueryRow(ctx, `
		INSERT INTO queue_entries (appointment_id, patient_id, department_id, doctor_id, priority, status)
		VALUES ($1, $2, $3, $4, $5, 'waiting')
		RETURNING id, queue_number, status, checked_in_at, created_at, updated_at`,
		q.AppointmentID, q.PatientID, q.DepartmentID, q.DoctorID, q.Priority,
	)
	if err := row.Scan(&q.ID, &q.QueueNumber, &q.Status, &q.CheckedInAt, &q.CreatedAt, &q.UpdatedAt); err != nil {
		if isUniqueViolation(err) {
			return apperr.Conflict("this appointment has already been checked in")
		}
		if isForeignKeyViolation(err) {
			return apperr.Validation("appointment, patient, or department does not exist", nil)
		}
		return fmt.Errorf("queue_pg: create: %w", err)
	}
	return nil
}

func (r *QueuePG) GetByID(ctx context.Context, id string) (*domain.QueueEntry, error) {
	row := r.pool.Querier(ctx).QueryRow(ctx, `
		SELECT q.id, q.appointment_id, q.patient_id, q.department_id, q.doctor_id, q.priority, q.status,
		       q.queue_number, q.checked_in_at, q.called_at, q.started_at, q.completed_at, q.created_at, q.updated_at,
		       p.first_name || ' ' || p.last_name
		FROM queue_entries q
		JOIN patients p ON p.id = q.patient_id
		WHERE q.id = $1`, id)
	return scanQueueEntry(row)
}

// CallNext locks and claims the next patient to be seen in a department.
// SELECT ... FOR UPDATE SKIP LOCKED means concurrent callers (two
// clinicians hitting "call next" at the same instant, or two API replicas)
// never select the same row: whichever transaction gets there first locks
// it, the other skips past it to the next candidate instead of blocking or
// double-assigning. This is the single source of truth for queue
// concurrency; Redis is a read cache layered on top of it.
func (r *QueuePG) CallNext(ctx context.Context, departmentID string, doctorID *string) (*domain.QueueEntry, error) {
	row := r.pool.Querier(ctx).QueryRow(ctx, `
		UPDATE queue_entries
		SET status = 'called', doctor_id = COALESCE($2, doctor_id), called_at = now(), updated_at = now()
		WHERE id = (
			SELECT id FROM queue_entries
			WHERE department_id = $1 AND status = 'waiting'
			ORDER BY priority DESC, checked_in_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING id, appointment_id, patient_id, department_id, doctor_id, priority, status,
		          queue_number, checked_in_at, called_at, started_at, completed_at, created_at, updated_at`,
		departmentID, doctorID,
	)

	var q domain.QueueEntry
	err := row.Scan(&q.ID, &q.AppointmentID, &q.PatientID, &q.DepartmentID, &q.DoctorID, &q.Priority, &q.Status,
		&q.QueueNumber, &q.CheckedInAt, &q.CalledAt, &q.StartedAt, &q.CompletedAt, &q.CreatedAt, &q.UpdatedAt)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("queue_pg: call next scan: %w", err)
	}

	if name, err := r.patientName(ctx, q.PatientID); err == nil {
		q.PatientName = name
	}
	return &q, nil
}

func (r *QueuePG) patientName(ctx context.Context, patientID string) (string, error) {
	var name string
	err := r.pool.Querier(ctx).QueryRow(ctx,
		`SELECT first_name || ' ' || last_name FROM patients WHERE id = $1`, patientID).Scan(&name)
	return name, err
}

func (r *QueuePG) UpdateStatus(ctx context.Context, id string, status domain.QueueEntryStatus) error {
	var timestampCol string
	switch status {
	case domain.QueueInProgress:
		timestampCol = "started_at = now(),"
	case domain.QueueCompleted, domain.QueueSkipped:
		timestampCol = "completed_at = now(),"
	}

	cmd, err := r.pool.Querier(ctx).Exec(ctx, fmt.Sprintf(`
		UPDATE queue_entries SET status = $1, %s updated_at = now() WHERE id = $2`, timestampCol),
		status, id,
	)
	if err != nil {
		return fmt.Errorf("queue_pg: update status: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return apperr.NotFound("queue entry")
	}
	return nil
}

func (r *QueuePG) ListWaiting(ctx context.Context, departmentID string) ([]domain.QueueEntry, error) {
	rows, err := r.pool.Querier(ctx).Query(ctx, `
		SELECT q.id, q.appointment_id, q.patient_id, q.department_id, q.doctor_id, q.priority, q.status,
		       q.queue_number, q.checked_in_at, q.called_at, q.started_at, q.completed_at, q.created_at, q.updated_at,
		       p.first_name || ' ' || p.last_name
		FROM queue_entries q
		JOIN patients p ON p.id = q.patient_id
		WHERE q.department_id = $1 AND q.status = 'waiting'
		ORDER BY q.priority DESC, q.checked_in_at ASC`, departmentID)
	if err != nil {
		return nil, fmt.Errorf("queue_pg: list waiting: %w", err)
	}
	defer rows.Close()

	var out []domain.QueueEntry
	for rows.Next() {
		q, err := scanQueueEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *q)
	}
	return out, rows.Err()
}

func (r *QueuePG) CountWaiting(ctx context.Context, departmentID string) (int, error) {
	var n int
	err := r.pool.Querier(ctx).QueryRow(ctx,
		`SELECT count(*) FROM queue_entries WHERE department_id = $1 AND status = 'waiting'`, departmentID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("queue_pg: count waiting: %w", err)
	}
	return n, nil
}

func scanQueueEntry(row rowScanner) (*domain.QueueEntry, error) {
	var q domain.QueueEntry
	err := row.Scan(&q.ID, &q.AppointmentID, &q.PatientID, &q.DepartmentID, &q.DoctorID, &q.Priority, &q.Status,
		&q.QueueNumber, &q.CheckedInAt, &q.CalledAt, &q.StartedAt, &q.CompletedAt, &q.CreatedAt, &q.UpdatedAt,
		&q.PatientName)
	if err != nil {
		if isNoRows(err) {
			return nil, apperr.NotFound("queue entry")
		}
		return nil, fmt.Errorf("queue_pg: scan: %w", err)
	}
	return &q, nil
}
