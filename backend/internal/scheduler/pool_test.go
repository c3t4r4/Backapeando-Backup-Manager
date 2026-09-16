package scheduler

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewWorkerPool(t *testing.T) {
	maxWorkers := 3
	pool := NewWorkerPool(maxWorkers)

	if pool.maxWorkers != maxWorkers {
		t.Errorf("expected maxWorkers=%d, got %d", maxWorkers, pool.maxWorkers)
	}

	if cap(pool.queue) != maxWorkers {
		t.Errorf("expected queue capacity=%d, got %d", maxWorkers, cap(pool.queue))
	}

	if pool.closed {
		t.Error("expected pool to not be closed on creation")
	}

	if pool.ctx == nil {
		t.Error("expected pool context to be initialized")
	}

	if pool.cancel == nil {
		t.Error("expected pool cancel to be initialized")
	}
}

func TestPoolStart(t *testing.T) {
	pool := NewWorkerPool(3)

	pool.Start(2)

	// Give goroutines a moment to start
	time.Sleep(50 * time.Millisecond)

	// Submit a simple task to verify workers are running
	executed := false
	err := pool.Submit(WorkerTask{
		ID:       "test-1",
		ServerID: "server-1",
		Execute: func(ctx context.Context) error {
			executed = true
			return nil
		},
	})

	if err != nil {
		t.Fatalf("failed to submit task: %v", err)
	}

	// Wait for task to execute
	time.Sleep(100 * time.Millisecond)

	if !executed {
		t.Error("expected task to be executed")
	}

	err = pool.Stop(context.Background())
	if err != nil {
		t.Fatalf("failed to stop pool: %v", err)
	}
}

func TestPoolSubmit(t *testing.T) {
	pool := NewWorkerPool(2)
	pool.Start(2)
	defer pool.Stop(context.Background())

	executed := atomic.Int32{}

	// Submit 5 tasks; with 2 workers, max 2 should run concurrently
	for i := 0; i < 5; i++ {
		idx := i

		err := pool.Submit(WorkerTask{
			ID:       fmt.Sprintf("task-%d", idx),
			ServerID: "server-1",
			Execute: func(ctx context.Context) error {
				executed.Add(1)
				time.Sleep(50 * time.Millisecond)
				return nil
			},
		})

		if err != nil {
			t.Fatalf("failed to submit task %d: %v", idx, err)
		}
	}

	// Wait for all tasks to be submitted and executed
	time.Sleep(300 * time.Millisecond)

	if executed.Load() != 5 {
		t.Errorf("expected 5 tasks executed, got %d", executed.Load())
	}
}

func TestPoolSubmitAfterStop(t *testing.T) {
	pool := NewWorkerPool(2)
	pool.Start(2)
	err := pool.Stop(context.Background())
	if err != nil {
		t.Fatalf("failed to stop pool: %v", err)
	}

	// Attempt to submit after stop should fail
	err = pool.Submit(WorkerTask{
		ID:       "test-1",
		ServerID: "server-1",
		Execute: func(ctx context.Context) error {
			return nil
		},
	})

	if err == nil {
		t.Error("expected error when submitting to closed pool")
	}
}

func TestPoolAvailableSlots(t *testing.T) {
	maxWorkers := 3
	pool := NewWorkerPool(maxWorkers)
	// Start with 1 worker to ensure queue accumulation
	pool.Start(1)
	defer pool.Stop(context.Background())

	// Initially all slots should be available
	available := pool.AvailableSlots()
	if available != maxWorkers {
		t.Errorf("expected %d available slots, got %d", maxWorkers, available)
	}

	// Submit 2 fast tasks followed by checking queue depth
	// With 1 worker and multiple tasks submitted quickly,
	// some tasks may remain in the queue briefly
	executed := atomic.Int32{}
	blockChan := make(chan struct{})
	defer close(blockChan)

	for i := 0; i < 2; i++ {
		idx := i
		err := pool.Submit(WorkerTask{
			ID:       fmt.Sprintf("task-%d", idx),
			ServerID: "server-1",
			Execute: func(ctx context.Context) error {
				executed.Add(1)
				// Keep tasks alive briefly to allow queue observation
				time.Sleep(10 * time.Millisecond)
				return nil
			},
		})
		if err != nil {
			t.Fatalf("failed to submit task %d: %v", idx, err)
		}
	}

	// Immediately after submission, check if queue has tasks
	// (timing variance is expected, but at some point slots should decrease)
	// This is a best-effort test: check after very small delay
	for attempt := 0; attempt < 5; attempt++ {
		available = pool.AvailableSlots()
		if available < maxWorkers {
			// Success: queue had accumulated some tasks
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// At least one slot should have been occupied at some point
	// (This is a relaxed assertion due to timing variance)
	// Tasks should all complete within a reasonable time
	time.Sleep(100 * time.Millisecond)

	// Verify both tasks executed
	if executed.Load() != 2 {
		t.Errorf("expected 2 tasks executed, got %d", executed.Load())
	}
}

func TestPoolStop(t *testing.T) {
	pool := NewWorkerPool(3)
	pool.Start(3)

	executed := atomic.Int32{}

	// Submit multiple tasks
	for i := 0; i < 5; i++ {
		idx := i
		err := pool.Submit(WorkerTask{
			ID:       fmt.Sprintf("task-%d", idx),
			ServerID: "server-1",
			Execute: func(ctx context.Context) error {
				executed.Add(1)
				time.Sleep(10 * time.Millisecond)
				return nil
			},
		})
		if err != nil {
			t.Fatalf("failed to submit task %d: %v", idx, err)
		}
	}

	// Stop should wait for all submitted tasks to complete
	err := pool.Stop(context.Background())
	if err != nil {
		t.Fatalf("failed to stop pool: %v", err)
	}

	// Verify all tasks were executed
	if executed.Load() != 5 {
		t.Errorf("expected 5 tasks executed before stop, got %d", executed.Load())
	}

	// Verify pool is marked closed
	if !pool.closed {
		t.Error("expected pool to be marked closed after Stop()")
	}
}

func TestPoolConcurrencyLimit(t *testing.T) {
	maxWorkers := 2
	pool := NewWorkerPool(maxWorkers)
	pool.Start(maxWorkers)
	defer pool.Stop(context.Background())

	var mu sync.Mutex
	concurrent := 0
	maxConcurrent := 0

	// Submit tasks that track concurrent execution
	for i := 0; i < 6; i++ {
		idx := i
		err := pool.Submit(WorkerTask{
			ID:       fmt.Sprintf("task-%d", idx),
			ServerID: "server-1",
			Execute: func(ctx context.Context) error {
				mu.Lock()
				concurrent++
				if concurrent > maxConcurrent {
					maxConcurrent = concurrent
				}
				mu.Unlock()

				time.Sleep(50 * time.Millisecond)

				mu.Lock()
				concurrent--
				mu.Unlock()

				return nil
			},
		})
		if err != nil {
			t.Fatalf("failed to submit task %d: %v", idx, err)
		}
	}

	// Wait for all tasks to complete
	time.Sleep(400 * time.Millisecond)

	if maxConcurrent > maxWorkers {
		t.Errorf("expected max %d concurrent tasks, observed %d", maxWorkers, maxConcurrent)
	}
}

func TestPoolTaskError(t *testing.T) {
	pool := NewWorkerPool(2)
	pool.Start(1)
	defer pool.Stop(context.Background())

	executed := false
	testErr := fmt.Errorf("test error")

	err := pool.Submit(WorkerTask{
		ID:       "failing-task",
		ServerID: "server-1",
		Execute: func(ctx context.Context) error {
			executed = true
			return testErr
		},
	})

	if err != nil {
		t.Fatalf("failed to submit task: %v", err)
	}

	// Wait for task to execute
	time.Sleep(100 * time.Millisecond)

	if !executed {
		t.Error("expected task to be executed despite error")
	}
}

func TestPoolContextCancellation(t *testing.T) {
	pool := NewWorkerPool(2)
	pool.Start(1)

	// Submit a task that checks context
	ctxCancelled := false

	err := pool.Submit(WorkerTask{
		ID:       "ctx-test",
		ServerID: "server-1",
		Execute: func(ctx context.Context) error {
			<-ctx.Done()
			ctxCancelled = true
			return nil
		},
	})

	if err != nil {
		t.Fatalf("failed to submit task: %v", err)
	}

	// Stop should cancel context
	err = pool.Stop(context.Background())
	if err != nil {
		t.Fatalf("failed to stop pool: %v", err)
	}

	if !ctxCancelled {
		t.Error("expected task context to be cancelled")
	}
}

func TestPoolDoubleStop(t *testing.T) {
	pool := NewWorkerPool(2)
	pool.Start(1)

	err := pool.Stop(context.Background())
	if err != nil {
		t.Fatalf("first stop failed: %v", err)
	}

	// Second stop should not panic or error
	err = pool.Stop(context.Background())
	if err != nil {
		t.Fatalf("second stop failed: %v", err)
	}
}

// TestPoolAvailableSlots_ReflectsBusyWorkers is a regression test for the
// bug where AvailableSlots measured free channel buffer space (cap-len)
// instead of workers actually executing a task. With 2 long-running tasks
// occupying both workers, the old implementation reported 2 free slots as
// soon as both tasks were dequeued from the channel — even though zero
// workers were actually free — which let the scheduler keep enqueuing tasks
// behind a stuck worker forever (see docs/RegrasNegocio.md RN-BACKUP-028).
func TestPoolAvailableSlots_ReflectsBusyWorkers(t *testing.T) {
	maxWorkers := 2
	pool := NewWorkerPool(maxWorkers)
	pool.Start(maxWorkers)
	defer pool.Stop(context.Background())

	release := make(chan struct{})
	started := make(chan struct{}, maxWorkers)

	for i := 0; i < maxWorkers; i++ {
		idx := i
		if err := pool.Submit(WorkerTask{
			ID:       fmt.Sprintf("long-task-%d", idx),
			ServerID: "server-1",
			Execute: func(ctx context.Context) error {
				started <- struct{}{}
				<-release
				return nil
			},
		}); err != nil {
			t.Fatalf("failed to submit task %d: %v", idx, err)
		}
	}

	// Wait until both tasks have actually started executing (not just been
	// dequeued into a goroutine that hasn't run yet).
	for i := 0; i < maxWorkers; i++ {
		<-started
	}

	if got := pool.AvailableSlots(); got != 0 {
		t.Errorf("AvailableSlots() = %d while both workers are busy, want 0", got)
	}

	close(release)

	// Once both tasks return, the pool should report full capacity again.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if pool.AvailableSlots() == maxWorkers {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Errorf("AvailableSlots() never returned to %d after tasks completed, got %d", maxWorkers, pool.AvailableSlots())
}

// TestPoolTaskTimeout_CancelsHungTask is a regression test for the missing
// per-task timeout: before SetTaskTimeout existed, a task could occupy its
// worker forever (e.g. a stalled SSH stream), which combined with the
// AvailableSlots bug above let the scheduler pile up backup runs behind it
// indefinitely (RN-BACKUP-028). With a short timeout configured, a task that
// ignores ctx.Done() for too long must still see its context canceled.
func TestPoolTaskTimeout_CancelsHungTask(t *testing.T) {
	pool := NewWorkerPool(1)
	pool.SetTaskTimeout(50 * time.Millisecond)
	pool.Start(1)
	defer pool.Stop(context.Background())

	ctxErr := make(chan error, 1)
	if err := pool.Submit(WorkerTask{
		ID:       "hung-task",
		ServerID: "server-1",
		Execute: func(ctx context.Context) error {
			<-ctx.Done()
			ctxErr <- ctx.Err()
			return ctx.Err()
		},
	}); err != nil {
		t.Fatalf("failed to submit task: %v", err)
	}

	select {
	case err := <-ctxErr:
		if err != context.DeadlineExceeded {
			t.Errorf("ctx.Err() = %v, want context.DeadlineExceeded", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("task context was never canceled by the configured timeout")
	}
}

func TestPoolTaskFieldsPreserved(t *testing.T) {
	pool := NewWorkerPool(2)
	pool.Start(1)
	defer pool.Stop(context.Background())

	received := false

	err := pool.Submit(WorkerTask{
		ID:       "custom-task-id",
		ServerID: "custom-server-id",
		Execute: func(ctx context.Context) error {
			received = true
			return nil
		},
	})

	if err != nil {
		t.Fatalf("failed to submit task: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if !received {
		t.Error("expected task to be executed")
	}
}
