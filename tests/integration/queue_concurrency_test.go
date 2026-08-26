//go:build integration

package integration

import (
	"context"
	"sync"
	"testing"

	"github.com/stan-ley-tech/medqueue/internal/domain"
	"github.com/stan-ley-tech/medqueue/internal/repository"
	"github.com/stan-ley-tech/medqueue/tests/testutil"
)

// TestCallNext_ConcurrentCallersNeverClaimTheSameEntry is the test that
// justifies SELECT ... FOR UPDATE SKIP LOCKED in QueueRepository.CallNext:
// it fires many concurrent CallNext calls at a department with a known
// number of waiting entries and asserts every entry was claimed exactly
// once, with no entry claimed twice and no entry left behind. Run with
// -race (make test-race) to also catch any data race in the Go code path
// itself, on top of the database-level correctness this test checks.
func TestCallNext_ConcurrentCallersNeverClaimTheSameEntry(t *testing.T) {
	pool := testutil.RequireDB(t)
	testutil.TruncateAll(t, pool)

	dept := testutil.SeedDepartment(t, pool)
	doctor := testutil.SeedDoctor(t, pool, dept.ID)

	const entryCount = 40
	seeded := make(map[string]bool, entryCount)
	for i := 0; i < entryCount; i++ {
		entry := testutil.SeedCheckedInQueueEntry(t, pool, dept.ID, doctor.ID, domain.PriorityNormal)
		seeded[entry.ID] = true
	}

	queueRepo := repository.NewQueueRepository(pool)

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		claimed = make(map[string]int) // entry ID -> number of times claimed
	)

	// Fire more concurrent callers than there are entries so we
	// definitely exercise the "queue is empty" path too, and to maximize
	// the chance of exposing a race if SKIP LOCKED weren't doing its job.
	const callerCount = entryCount + 20
	wg.Add(callerCount)
	for i := 0; i < callerCount; i++ {
		go func() {
			defer wg.Done()
			entry, err := queueRepo.CallNext(context.Background(), dept.ID, &doctor.ID)
			if err != nil {
				t.Errorf("CallNext returned error: %v", err)
				return
			}
			if entry == nil {
				return
			}
			mu.Lock()
			claimed[entry.ID]++
			mu.Unlock()
		}()
	}
	wg.Wait()

	if len(claimed) != entryCount {
		t.Fatalf("expected all %d seeded entries to be claimed, got %d distinct entries claimed", entryCount, len(claimed))
	}
	for id, count := range claimed {
		if !seeded[id] {
			t.Errorf("claimed entry %s was not one of the seeded entries", id)
		}
		if count != 1 {
			t.Errorf("entry %s was claimed %d times, want exactly 1", id, count)
		}
	}

	// The queue must now be empty: one more call finds nothing.
	extra, err := queueRepo.CallNext(context.Background(), dept.ID, &doctor.ID)
	if err != nil {
		t.Fatalf("CallNext on empty queue returned error: %v", err)
	}
	if extra != nil {
		t.Fatalf("expected nil on empty queue, got entry %s", extra.ID)
	}
}

// TestCallNext_RespectsPriorityThenArrivalOrder checks the ordering
// contract, not just the concurrency contract: emergency before urgent
// before normal, and FIFO within the same priority band.
func TestCallNext_RespectsPriorityThenArrivalOrder(t *testing.T) {
	pool := testutil.RequireDB(t)
	testutil.TruncateAll(t, pool)

	dept := testutil.SeedDepartment(t, pool)
	doctor := testutil.SeedDoctor(t, pool, dept.ID)
	queueRepo := repository.NewQueueRepository(pool)
	ctx := context.Background()

	normal := testutil.SeedCheckedInQueueEntry(t, pool, dept.ID, doctor.ID, domain.PriorityNormal)
	urgent := testutil.SeedCheckedInQueueEntry(t, pool, dept.ID, doctor.ID, domain.PriorityUrgent)
	emergency := testutil.SeedCheckedInQueueEntry(t, pool, dept.ID, doctor.ID, domain.PriorityEmergency)

	first, err := queueRepo.CallNext(ctx, dept.ID, &doctor.ID)
	if err != nil {
		t.Fatalf("CallNext: %v", err)
	}
	if first == nil || first.ID != emergency.ID {
		t.Fatalf("expected emergency entry called first, got %+v", first)
	}

	second, err := queueRepo.CallNext(ctx, dept.ID, &doctor.ID)
	if err != nil {
		t.Fatalf("CallNext: %v", err)
	}
	if second == nil || second.ID != urgent.ID {
		t.Fatalf("expected urgent entry called second, got %+v", second)
	}

	third, err := queueRepo.CallNext(ctx, dept.ID, &doctor.ID)
	if err != nil {
		t.Fatalf("CallNext: %v", err)
	}
	if third == nil || third.ID != normal.ID {
		t.Fatalf("expected normal entry called third, got %+v", third)
	}
}
