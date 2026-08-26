package service

import (
	"context"

	"github.com/stan-ley-tech/medqueue/internal/domain"
	"github.com/stan-ley-tech/medqueue/internal/repository"
)

type DoctorService struct {
	repo  repository.DoctorRepository
	audit *AuditService
}

func NewDoctorService(repo repository.DoctorRepository, audit *AuditService) *DoctorService {
	return &DoctorService{repo: repo, audit: audit}
}

func (s *DoctorService) Create(ctx context.Context, actor Actor, d *domain.Doctor) error {
	if err := s.repo.Create(ctx, d); err != nil {
		return err
	}
	s.audit.Record(ctx, actor.UserID, actor.Role, "doctor.created", "doctor", d.ID, map[string]any{"department_id": d.DepartmentID})
	return nil
}

func (s *DoctorService) Get(ctx context.Context, id string) (*domain.Doctor, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *DoctorService) Update(ctx context.Context, actor Actor, d *domain.Doctor) error {
	if err := s.repo.Update(ctx, d); err != nil {
		return err
	}
	s.audit.Record(ctx, actor.UserID, actor.Role, "doctor.updated", "doctor", d.ID, nil)
	return nil
}

func (s *DoctorService) List(ctx context.Context, filter domain.DoctorFilter) (domain.PagedResult[domain.Doctor], error) {
	items, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return domain.PagedResult[domain.Doctor]{}, err
	}
	return domain.PagedResult[domain.Doctor]{Items: items, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}
