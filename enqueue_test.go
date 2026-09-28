package jobq_test

import (
	"testing"

	"github.com/adtoba/jobq"
	"github.com/adtoba/jobq/internal/testdb"
)

type emailArgs struct {
	To string
}

func TestEnqueue_Defaults(t *testing.T) {
	db := testdb.New(t)
	kind := "send_email"
	args := emailArgs{
		To: "user@example.com",
	}

	client, err := jobq.NewClient(db)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	id, err := client.Enqueue(t.Context(), kind, args, nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if id == 0 {
		t.Fatalf("ID should not be 0")
	}

	var (
		k           string
		queue       string
		state       string
		attempt     int
		maxAttempts int
	)

	var gotArgs emailArgs

	err = db.QueryRow(t.Context(), "SELECT kind, queue, state, attempt, max_attempts, args FROM jobs WHERE id = $1", id).Scan(&k, &queue, &state, &attempt, &maxAttempts, &gotArgs)
	if err != nil {
		t.Fatalf("DB query failed: %v", err)
	}

	if k != "send_email" {
		t.Errorf("kind = %q, want %q", k, "send_email")
	}

	if queue != "default" {
		t.Errorf("queue = %q, want %q", queue, "default")
	}

	if gotArgs != args {
		t.Errorf("args = %+v, want %+v", gotArgs, args)
	}

	if state != "available" {
		t.Errorf("state = %q, want %q", state, "available")
	}

	if attempt != 0 {
		t.Errorf("attempt = %d, want %d", attempt, 0)
	}

	if maxAttempts != 20 {
		t.Errorf("maxAttempts = %d, want %d", maxAttempts, 20)
	}

}
