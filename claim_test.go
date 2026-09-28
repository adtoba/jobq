package jobq

import (
	"errors"
	"testing"
	"time"

	"github.com/adtoba/jobq/internal/testdb"
)

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
