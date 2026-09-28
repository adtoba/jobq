package jobq

import (
	"testing"
	"time"

	"github.com/adtoba/jobq/internal/testdb"
)

func TestComplete_MarksJobCompleted(t *testing.T) {
	db := testdb.New(t)
	client, err := NewClient(db)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	workerID := "worker-1"
	args := map[string]any{
		"to": "user@example.com",
	}

	_, err = client.Enqueue(t.Context(), "send_email", args, &EnqueueOpts{})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	job, err := client.claim(t.Context(), "default", workerID)
	if err != nil {
		t.Fatalf("worker %s, error: %v", workerID, err)
	}

	err = client.complete(t.Context(), job.ID, workerID)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}

	var (
		state       string
		finalizedAt *time.Time
	)

	err = db.QueryRow(
		t.Context(),
		"SELECT state, finalized_at FROM jobs WHERE id = $1",
		job.ID,
	).Scan(&state, &finalizedAt)

	if err != nil {
		t.Fatalf("DB query failed: %v", err)
	}

	if state != "completed" {
		t.Errorf("got state = %q, want %q", state, "completed")
	}

	if finalizedAt == nil {
		t.Errorf("finalized_at is NULL, want a timestamp")
	}
}
