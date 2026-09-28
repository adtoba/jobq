package jobq_test

import (
	"errors"
	"testing"

	"github.com/adtoba/jobq"
	"github.com/adtoba/jobq/internal/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

type emailArgs struct {
	To string
}

func countJobs(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var count int
	err := pool.QueryRow(
		t.Context(),
		"SELECT count(*) FROM jobs",
	).Scan(&count)

	if err != nil {
		t.Fatalf("count jobs failed: %v", err)
	}
	return count
}

func TestEnqueue_Defaults(t *testing.T) {
	db := testdb.New(t)
	args := emailArgs{
		To: "user@example.com",
	}

	client, err := jobq.NewClient(db)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	id, err := client.Enqueue(t.Context(), "send_email", args, nil)

	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if id == 0 {
		t.Fatalf("ID should not be 0")
	}

	var (
		kind        string
		queue       string
		state       string
		attempt     int
		maxAttempts int
	)

	var gotArgs emailArgs

	err = db.QueryRow(
		t.Context(),
		"SELECT kind, queue, state, attempt, max_attempts, args FROM jobs WHERE id = $1", id).Scan(
		&kind, &queue, &state, &attempt, &maxAttempts, &gotArgs,
	)

	if err != nil {
		t.Fatalf("DB query failed: %v", err)
	}

	if kind != "send_email" {
		t.Errorf("kind = %q, want %q", kind, "send_email")
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

func TestEnqueue_EmptyKind(t *testing.T) {
	pool := testdb.New(t)
	args := emailArgs{
		To: "user@example.com",
	}

	client, err := jobq.NewClient(pool)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	id, err := client.Enqueue(t.Context(), "", args, nil)
	if !errors.Is(err, jobq.ErrEmptyKind) {
		t.Fatalf("got error %v, want %v", err, jobq.ErrEmptyKind)
	}

	if id != 0 {
		t.Errorf("id should be 0")
	}

	count := countJobs(t, pool)
	if count != 0 {
		t.Fatalf("got %d jobs in the table, want 0", count)
	}
}

func TestEnqueue_NilArgs(t *testing.T) {
	tests := []struct {
		name string
		args any
	}{
		{name: "untyped nil", args: nil},
		{name: "typed nil pointer", args: (*emailArgs)(nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testdb.New(t)
			client, err := jobq.NewClient(db)
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}

			id, err := client.Enqueue(t.Context(), "send_email", tt.args, nil)
			if !errors.Is(err, jobq.ErrEmptyArgs) {
				t.Fatalf("got error %v, want %v", err, jobq.ErrEmptyArgs)
			}

			if id != 0 {
				t.Errorf("id = %d, want 0", id)
			}

			count := countJobs(t, db)
			if count != 0 {
				t.Fatalf("got %d jobs in the table, want 0", count)
			}
		})
	}
}
