package service

import (
	"context"
	"log/slog"

	"github.com/stan-ley-tech/medqueue/internal/domain"
	"github.com/stan-ley-tech/medqueue/internal/repository"
)

// AuditService records who did what to which entity. Callers should
// invoke Record from inside the same database transaction as the change
// it describes wherever the write path already opens one, so the audit
// trail and the mutation it documents commit or roll back together.
type AuditService struct {
	repo repository.AuditRepository
	log  *slog.Logger
}

func NewAuditService(repo repository.AuditRepository, log *slog.Logger) *AuditService {
	return &AuditService{repo: repo, log: log}
}

func (s *AuditService) Record(ctx context.Context, actorID string, actorRole domain.Role, action, entityType, entityID string, metadata map[string]any) {
	entry := &domain.AuditLog{
		ActorID:    actorID,
		ActorRole:  actorRole,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Metadata:   metadata,
	}
	if err := s.repo.Create(ctx, entry); err != nil {
		// Audit logging must never break the primary operation; a failure
		// here is surfaced to logs/alerting instead of the caller.
		s.log.Error("audit: failed to record entry", "action", action, "entity_type", entityType, "entity_id", entityID, "error", err)
	}
}

func (s *AuditService) List(ctx context.Context, filter domain.AuditFilter) (domain.PagedResult[domain.AuditLog], error) {
	items, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return domain.PagedResult[domain.AuditLog]{}, err
	}
	return domain.PagedResult[domain.AuditLog]{Items: items, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}
