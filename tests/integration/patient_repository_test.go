//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stan-ley-tech/medqueue/internal/apperr"
	"github.com/stan-ley-tech/medqueue/internal/domain"
	"github.com/stan-ley-tech/medqueue/internal/repository"
	"github.com/stan-ley-tech/medqueue/tests/testutil"
)

func TestPatientRepository_CreateGetUpdate(t *testing.T) {
	pool := testutil.RequireDB(t)
	testutil.TruncateAll(t, pool)
	repo := repository.NewPatientRepository(pool)
	ctx := context.Background()

	p := &domain.Patient{
		MedicalRecordNumber: "MRN-0001",
		FirstName:           "Ada",
		LastName:            "Lovelace",
		DateOfBirth:         time.Date(1990, 5, 12, 0, 0, 0, 0, time.UTC),
		Phone:               "+15550001111",
	}
	if err := repo.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.ID == "" {
		t.Fatal("expected Create to populate ID")
	}

	got, err := repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.FirstName != "Ada" || got.LastName != "Lovelace" {
		t.Errorf("unexpected patient: %+v", got)
	}

	got.LastName = "King"
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}

	updated, err := repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if updated.LastName != "King" {
		t.Errorf("expected updated last name %q, got %q", "King", updated.LastName)
	}
}

func TestPatientRepository_DuplicateMRNConflicts(t *testing.T) {
	pool := testutil.RequireDB(t)
	testutil.TruncateAll(t, pool)
	repo := repository.NewPatientRepository(pool)
	ctx := context.Background()

	base := domain.Patient{
		MedicalRecordNumber: "MRN-DUP",
		FirstName:           "First",
		LastName:            "Patient",
		DateOfBirth:         time.Date(1985, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	p1 := base
	if err := repo.Create(ctx, &p1); err != nil {
		t.Fatalf("Create first patient: %v", err)
	}

	p2 := base
	p2.LastName = "Duplicate"
	err := repo.Create(ctx, &p2)
	if err == nil {
		t.Fatal("expected duplicate MRN to be rejected")
	}
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != apperr.CodeConflict {
		t.Fatalf("expected a CONFLICT apperr, got %v", err)
	}
}

func TestPatientRepository_ListSearchAndPagination(t *testing.T) {
	pool := testutil.RequireDB(t)
	testutil.TruncateAll(t, pool)
	repo := repository.NewPatientRepository(pool)
	ctx := context.Background()

	names := []string{"Alice Zephyr", "Bob Zephyr", "Carol Zephyr"}
	for i, full := range names {
		parts := splitName(full)
		p := &domain.Patient{
			MedicalRecordNumber: "MRN-SEARCH-" + parts[0],
			FirstName:           parts[0],
			LastName:            parts[1],
			DateOfBirth:         time.Date(1990, 1, 1+i, 0, 0, 0, 0, time.UTC),
		}
		if err := repo.Create(ctx, p); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	filter := domain.PatientFilter{Search: "Zephyr", Page: domain.Page{Limit: 2, Offset: 0}}
	page1, total, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("List page 1: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total=3, got %d", total)
	}
	if len(page1) != 2 {
		t.Fatalf("expected 2 items on page 1, got %d", len(page1))
	}

	filter.Offset = 2
	page2, _, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("List page 2: %v", err)
	}
	if len(page2) != 1 {
		t.Fatalf("expected 1 item on page 2, got %d", len(page2))
	}
}

func splitName(full string) [2]string {
	for i := 0; i < len(full); i++ {
		if full[i] == ' ' {
			return [2]string{full[:i], full[i+1:]}
		}
	}
	return [2]string{full, ""}
}
