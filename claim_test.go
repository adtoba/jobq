package jobq

import (
	"testing"

	"github.com/adtoba/jobq/internal/testdb"
)

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
