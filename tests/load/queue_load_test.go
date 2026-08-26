//go:build load

package load

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stan-ley-tech/medqueue/internal/domain"
	"github.com/stan-ley-tech/medqueue/internal/repository"
	"github.com/stan-ley-tech/medqueue/tests/testutil"
)

// TestQueueLoad_CallNextThroughputUnderContention seeds a large waiting
// queue and drains it with many concurrent "call next" workers — the
// same access pattern as a busy multi-clinician department at opening
// time. It reports throughput and, more importantly, re-asserts the
// correctness property from the integration concurrency test (no entry
// claimed twice, none left behind) at a scale where a locking bug would
// be far more likely to show up than with a handful of goroutines.
//
// Run via `make test-load`, against docker compose's postgres service.
func TestQueueLoad_CallNextThroughputUnderContention(t *testing.T) {
	pool := testutil.RequireDB(t)
	testutil.TruncateAll(t, pool)

	dept := testutil.SeedDepartment(t, pool)
	doctor := testutil.SeedDoctor(t, pool, dept.ID)

	const entryCount = 500
	const workerCount = 50

	seeded := make(map[string]bool, entryCount)
	var seedMu sync.Mutex
	var seedWg sync.WaitGroup
	seedWg.Add(entryCount)
	for i := 0; i < entryCount; i++ {
		go func() {
			defer seedWg.Done()
			entry := testutil.SeedCheckedInQueueEntry(t, pool, dept.ID, doctor.ID, domain.QueuePriority(i%3))
			seedMu.Lock()
			seeded[entry.ID] = true
			seedMu.Unlock()
		}()
	}
	seedWg.Wait()

	queueRepo := repository.NewQueueRepository(pool)

	var (
		claimedMu sync.Mutex
		claimed   = make(map[string]int, entryCount)
		wg        sync.WaitGroup
	)

	start := time.Now()
	wg.Add(workerCount)
	for w := 0; w < workerCount; w++ {
		go func() {
			defer wg.Done()
			for {
				entry, err := queueRepo.CallNext(context.Background(), dept.ID, &doctor.ID)
				if err != nil {
					t.Errorf("CallNext: %v", err)
					return
				}
				if entry == nil {
					return // queue drained
				}
				claimedMu.Lock()
				claimed[entry.ID]++
				claimedMu.Unlock()
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)

	if len(claimed) != entryCount {
		t.Fatalf("expected %d entries claimed, got %d", entryCount, len(claimed))
	}
	for id, count := range claimed {
		if !seeded[id] {
			t.Errorf("claimed unknown entry %s", id)
		}
		if count != 1 {
			t.Errorf("entry %s claimed %d times, want 1", id, count)
		}
	}

	throughput := float64(entryCount) / elapsed.Seconds()
	t.Logf("drained %d entries with %d concurrent workers in %s (%.1f calls/sec)", entryCount, workerCount, elapsed, throughput)
}
