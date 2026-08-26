package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/stan-ley-tech/medqueue/internal/db"
	"github.com/stan-ley-tech/medqueue/internal/domain"
)

type AuditPG struct{ pgStore }

func NewAuditRepository(pool *db.Pool) *AuditPG {
	return &AuditPG{pgStore{pool}}
}

func (r *AuditPG) Create(ctx context.Context, a *domain.AuditLog) error {
	metadata, err := json.Marshal(a.Metadata)
	if err != nil {
		return fmt.Errorf("audit_pg: marshal metadata: %w", err)
	}

	var actorID any
	if a.ActorID != "" {
		actorID = a.ActorID
	}

	row := r.pool.Querier(ctx).QueryRow(ctx, `
		INSERT INTO audit_logs (actor_id, actor_role, action, entity_type, entity_id, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`,
		actorID, a.ActorRole, a.Action, a.EntityType, a.EntityID, metadata,
	)
	if err := row.Scan(&a.ID, &a.CreatedAt); err != nil {
		return fmt.Errorf("audit_pg: create: %w", err)
	}
	return nil
}

func (r *AuditPG) List(ctx context.Context, filter domain.AuditFilter) ([]domain.AuditLog, int, error) {
	filter.NormalizeDefaults()

	const base = `FROM audit_logs
		WHERE ($1 = '' OR entity_type = $1)
		AND ($2 = '' OR entity_id = $2)
		AND ($3 = '' OR actor_id = $3::uuid)`

	var total int
	if err := r.pool.Querier(ctx).QueryRow(ctx, `SELECT count(*) `+base,
		filter.EntityType, filter.EntityID, filter.ActorID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("audit_pg: count: %w", err)
	}

	rows, err := r.pool.Querier(ctx).Query(ctx, `
		SELECT id, coalesce(actor_id::text, ''), actor_role, action, entity_type, entity_id, metadata, created_at `+base+`
		ORDER BY created_at DESC LIMIT $4 OFFSET $5`,
		filter.EntityType, filter.EntityID, filter.ActorID, filter.Limit, filter.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("audit_pg: list: %w", err)
	}
	defer rows.Close()

	var out []domain.AuditLog
	for rows.Next() {
		var a domain.AuditLog
		var metadata []byte
		if err := rows.Scan(&a.ID, &a.ActorID, &a.ActorRole, &a.Action, &a.EntityType, &a.EntityID, &metadata, &a.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("audit_pg: scan: %w", err)
		}
		if len(metadata) > 0 {
			if err := json.Unmarshal(metadata, &a.Metadata); err != nil {
				return nil, 0, fmt.Errorf("audit_pg: unmarshal metadata: %w", err)
			}
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("audit_pg: rows: %w", err)
	}
	return out, total, nil
}
