package service

import (
	"context"

	"github.com/stan-ley-tech/medqueue/internal/domain"
	"github.com/stan-ley-tech/medqueue/internal/repository"
)

type PatientService struct {
	repo  repository.PatientRepository
	audit *AuditService
}

func NewPatientService(repo repository.PatientRepository, audit *AuditService) *PatientService {
	return &PatientService{repo: repo, audit: audit}
}

func (s *PatientService) Create(ctx context.Context, actor Actor, p *domain.Patient) error {
	if err := s.repo.Create(ctx, p); err != nil {
		return err
	}
	s.audit.Record(ctx, actor.UserID, actor.Role, "patient.created", "patient", p.ID, map[string]any{"mrn": p.MedicalRecordNumber})
	return nil
}

func (s *PatientService) Get(ctx context.Context, id string) (*domain.Patient, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *PatientService) Update(ctx context.Context, actor Actor, p *domain.Patient) error {
	if err := s.repo.Update(ctx, p); err != nil {
		return err
	}
	s.audit.Record(ctx, actor.UserID, actor.Role, "patient.updated", "patient", p.ID, nil)
	return nil
}

func (s *PatientService) List(ctx context.Context, filter domain.PatientFilter) (domain.PagedResult[domain.Patient], error) {
	items, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return domain.PagedResult[domain.Patient]{}, err
	}
	return domain.PagedResult[domain.Patient]{Items: items, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}
