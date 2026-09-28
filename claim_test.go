package jobq

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/adtoba/jobq/internal/testdb"
)

func TestClaim_ConcurrentWorkersClaimOnce(t *testing.T) {
	const workers = 20
	db := testdb.New(t)
	client, err := NewClient(db)
	if err != nil {
		t.Fatalf("NewClient : %v", err)
	}

	args := map[string]any{
		"to": "user@example.com",
	}

	id, err := client.Enqueue(t.Context(), "send_email", args, &EnqueueOpts{
		Priority: 0,
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	var readyWg sync.WaitGroup
	var wins, noJobs atomic.Int64

	for i := range workers {
		wg.Add(1)
		readyWg.Add(1)
		go func() {
			defer wg.Done()

			if _, err := db.Exec(t.Context(), "SELECT 1"); err != nil {
				t.Errorf("warm-up query: %v", err)
			}

			readyWg.Done()

			<-start

			workerID := fmt.Sprintf("worker-%d", i)

			_, err := client.claim(t.Context(), "default", workerID)

			switch {
			case err == nil:
				wins.Add(1)
			case errors.Is(err, errNoJobs):
				noJobs.Add(1)
			default:
				t.Errorf("worker %s, error: %v", workerID, err)
			}
		}()
	}
	readyWg.Wait()
	close(start)
	wg.Wait()

	if wins.Load() != 1 {
		t.Errorf("got wins %d, want %d", wins.Load(), 1)
	}

	if noJobs.Load() != workers-1 {
		t.Errorf("got no jobs %d, want %d", noJobs.Load(), workers-1)
	}

	var attempt int

	err = db.QueryRow(
		t.Context(),
		"SELECT attempt FROM jobs WHERE id = $1",
		id,
	).Scan(&attempt)
	if err != nil {
		t.Fatalf("DB query failed: %v", err)
	}

	if attempt != 1 {
		t.Errorf("attempt = %d, want %d", attempt, 1)
	}
}

func TestClaim_HighestPriorityFirst(t *testing.T) {
	workerID := "worker-1"

	db := testdb.New(t)
	client, err := NewClient(db)
	if err != nil {
		t.Fatalf("NewClient : %v", err)
	}

	args := map[string]any{
		"to": "user@example.com",
	}

	lowID, err := client.Enqueue(t.Context(), "send_email", args, &EnqueueOpts{
		Priority: 0,
		Queue:    "default",
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	highID, err := client.Enqueue(t.Context(), "send_email", args, &EnqueueOpts{
		Priority: 10,
		Queue:    "default",
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	job1, err := client.claim(t.Context(), "default", workerID)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	if job1.ID != highID {
		t.Errorf("first claim = job %d, want job %d (high priority)", job1.ID, highID)
	}

	job2, err := client.claim(t.Context(), "default", workerID)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	if job2.ID != lowID {
		t.Errorf("first claim = job %d, want job %d (low priority)", job2.ID, lowID)
	}

	job3, err := client.claim(t.Context(), "default", workerID)
	if !errors.Is(err, errNoJobs) {
		t.Fatalf("third claim error = %v, want %v", err, errNoJobs)
	}

	if job3 != nil {
		t.Fatalf("expected nil job")
	}
}

func TestClaim_EmptyQueue(t *testing.T) {
	workerID := "worker-1"

	db := testdb.New(t)
	client, err := NewClient(db)
	if err != nil {
		t.Fatalf("NewClient : %v", err)
	}

	job, err := client.claim(t.Context(), "default", workerID)
	if !errors.Is(err, errNoJobs) {
		t.Fatalf("got %v, want %v", err, errNoJobs)
	}

	if job != nil {
		t.Fatalf("claim returned job %d, want nil", job.ID)
	}
}

func TestClaim_SkipsIneligibleJobs(t *testing.T) {
	tests := []struct {
		name string
		opts *EnqueueOpts
	}{
		{name: "scheduled", opts: &EnqueueOpts{
			RunAt: time.Now().Add(1 * time.Hour),
		}},
		{name: "differentQueue", opts: &EnqueueOpts{
			Queue: "emails",
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testdb.New(t)
			client, err := NewClient(db)

			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}

			args := map[string]any{"to": "user@example.com"}

			_, err = client.Enqueue(t.Context(), "send_email", args, tt.opts)
			if err != nil {
				t.Fatalf("enqueue: %v", err)
			}

			job, err := client.claim(t.Context(), "default", "worker-1")
			if !errors.Is(err, errNoJobs) {
				t.Fatalf("claim error = %v, want %v", err, errNoJobs)
			}

			if job != nil {
				t.Fatalf("claim returned job %d, want nil", job.ID)
			}

		})
	}
}

func TestClaim_ClaimsAvailableJob(t *testing.T) {
	workerID := "worker-1"

	db := testdb.New(t)

	client, err := NewClient(db)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	args := map[string]any{
		"to": "user@example.com",
	}

	id, err := client.Enqueue(
		t.Context(),
		"send_email",
		args,
		nil,
	)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	if id == 0 {
		t.Fatalf("Enqueue returned id %d", id)
	}

	job, err := client.claim(t.Context(), "default", workerID)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	var (
		state    string
		lockedBy *string
	)

	err = db.QueryRow(
		t.Context(),
		"SELECT state, locked_by FROM jobs WHERE id = $1",
		id,
	).Scan(&state, &lockedBy)

	if err != nil {
		t.Fatalf("DB query failed: %v", err)
	}

	if state != "running" {
		t.Errorf("got %q, want %q", state, "running")
	}

	if lockedBy != nil {
		if *lockedBy != workerID {
			t.Errorf("got %q, want %q", *lockedBy, workerID)
		}
	} else {
		t.Errorf("locked_by is nil")
	}

	if job == nil {
		t.Fatalf("claim returned nil job, want a job")
	}

	if job.ID != id {
		t.Errorf("job.ID = %d, want %d", job.ID, id)
	}

	if job.Kind != "send_email" {
		t.Errorf("job.Kind = %q, want %q", job.Kind, "send_email")
	}

	if job.Attempt != 1 {
		t.Errorf("job.Attempt = %d, want %d", job.Attempt, 1)
	}

}
