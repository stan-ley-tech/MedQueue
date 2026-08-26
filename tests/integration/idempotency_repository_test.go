//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stan-ley-tech/medqueue/internal/repository"
	"github.com/stan-ley-tech/medqueue/tests/testutil"
)

func TestIdempotencyRepository_ReserveThenReplay(t *testing.T) {
	pool := testutil.RequireDB(t)
	testutil.TruncateAll(t, pool)
	repo := repository.NewIdempotencyRepository(pool)
	ctx := context.Background()

	key := "test-key-1"

	found, status, body, err := repo.Reserve(ctx, key, "/api/v1/appointments", "hash-1", time.Hour)
	if err != nil {
		t.Fatalf("Reserve (first): %v", err)
	}
	if found {
		t.Fatal("expected first Reserve to report found=false")
	}
	if status != 0 || body != nil {
		t.Fatalf("expected empty response on first reservation, got status=%d body=%s", status, body)
	}

	if err := repo.Complete(ctx, key, 201, []byte(`{"id":"abc"}`)); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	found, status, body, err = repo.Reserve(ctx, key, "/api/v1/appointments", "hash-1", time.Hour)
	if err != nil {
		t.Fatalf("Reserve (replay): %v", err)
	}
	if !found {
		t.Fatal("expected second Reserve with the same key to report found=true")
	}
	if status != 201 {
		t.Errorf("expected replayed status 201, got %d", status)
	}
	if string(body) != `{"id":"abc"}` {
		t.Errorf("expected replayed body to match original, got %s", body)
	}
}

func TestIdempotencyRepository_ConcurrentReserveOnlyOneWinsPlaceholder(t *testing.T) {
	pool := testutil.RequireDB(t)
	testutil.TruncateAll(t, pool)
	repo := repository.NewIdempotencyRepository(pool)
	ctx := context.Background()

	key := "test-key-concurrent"

	const attempts = 10
	results := make(chan bool, attempts)
	for i := 0; i < attempts; i++ {
		go func() {
			found, _, _, err := repo.Reserve(ctx, key, "/x", "hash", time.Hour)
			if err != nil {
				results <- false
				return
			}
			results <- !found
		}()
	}

	firstReservations := 0
	for i := 0; i < attempts; i++ {
		if <-results {
			firstReservations++
		}
	}
	if firstReservations != 1 {
		t.Fatalf("expected exactly 1 goroutine to win the first reservation, got %d", firstReservations)
	}
}
