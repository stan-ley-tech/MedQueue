package service

import (
	"context"

	"github.com/stan-ley-tech/medqueue/internal/domain"
	"github.com/stan-ley-tech/medqueue/internal/repository"
)

type DepartmentService struct {
	repo  repository.DepartmentRepository
	audit *AuditService
}

func NewDepartmentService(repo repository.DepartmentRepository, audit *AuditService) *DepartmentService {
	return &DepartmentService{repo: repo, audit: audit}
}

func (s *DepartmentService) Create(ctx context.Context, actor Actor, d *domain.Department) error {
	if err := s.repo.Create(ctx, d); err != nil {
		return err
	}
	s.audit.Record(ctx, actor.UserID, actor.Role, "department.created", "department", d.ID, map[string]any{"name": d.Name})
	return nil
}

func (s *DepartmentService) Get(ctx context.Context, id string) (*domain.Department, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *DepartmentService) Update(ctx context.Context, actor Actor, d *domain.Department) error {
	if err := s.repo.Update(ctx, d); err != nil {
		return err
	}
	s.audit.Record(ctx, actor.UserID, actor.Role, "department.updated", "department", d.ID, nil)
	return nil
}

func (s *DepartmentService) List(ctx context.Context, filter domain.DepartmentFilter) (domain.PagedResult[domain.Department], error) {
	items, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return domain.PagedResult[domain.Department]{}, err
	}
	return domain.PagedResult[domain.Department]{Items: items, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}
