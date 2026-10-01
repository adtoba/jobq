package jobq

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/adtoba/jobq/internal/testdb"
)

func TestWorker_RunsEveryJobOnce(t *testing.T) {
	const jobCount = 100

	db := testdb.New(t)
	client, err := NewClient(db)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx := t.Context()

	for i := range jobCount {
		args := map[string]int{
			"n": i,
		}
		_, err := client.Enqueue(ctx, "send_email", args, &EnqueueOpts{})
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}
	}

	var mu sync.Mutex
	seen := make(map[int64]int)

	handler := func(ctx context.Context, job *Job) error {
		mu.Lock()
		defer mu.Unlock()
		seen[job.ID]++
		return nil
	}

	worker := NewWorker(client, &WorkerConfig{
		ID:           "test-worker",
		PollInterval: time.Millisecond * 10,
	})

	err = worker.Register("send_email", handler)
	if err != nil {
		t.Fatalf("worker: %v", err)
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- worker.Run(runCtx)
	}()

	deadline := time.Now().Add(time.Second * 10)
	for {
		var completed int
		err = db.QueryRow(ctx, "SELECT count(*) FROM jobs WHERE state = 'completed'").Scan(&completed)
		if err != nil {
			t.Fatalf("DB query failed: %v", err)
		}
		if completed == jobCount {
			break
		}

		if time.Now().After(deadline) {
			t.Fatalf("timed out: %d of %d jobs completed", completed, jobCount)
		}

		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	err = <-done
	if err != nil {
		t.Errorf("Run: %v", err)
	}

	if len(seen) != jobCount {
		t.Errorf("handler ran %d distinct jobs, want %d", len(seen), jobCount)
	}

	for id, count := range seen {
		if count != 1 {
			t.Errorf("job %d ran %d times, want 1", id, count)
		}
	}

	var notCompleted int
	err = db.QueryRow(ctx, "SELECT count(*) FROM jobs WHERE state <> 'completed'").Scan(&notCompleted)
	if err != nil {
		t.Fatalf("DB query failed: %v", err)
	}

	if notCompleted != 0 {
		t.Errorf("%d jobs not completed, want 0", notCompleted)
	}

}
