package jobq_test

import (
	"errors"
	"testing"
	"time"

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

func newTestClient(t *testing.T) (*jobq.Client, *pgxpool.Pool) {
	t.Helper()
	db := testdb.New(t)
	client, err := jobq.NewClient(db)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client, db
}

// func TestEnqueue_NilOpts(t *testing.T) {
// 	tests := []struct {
// 		name string
// 		args any
// 	}{
// 		{name: "priority", args: 0},
// 		{name: "maxAttempts", args: 0},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			client, db := newTestClient(t)

// 		})
// 	}
// }

func TestEnqueue_EmptyOptsWithDefaults(t *testing.T) {
	client, db := newTestClient(t)

	args := emailArgs{
		To: "user@example.com",
	}

	opts := &jobq.EnqueueOpts{}

	id, err := client.Enqueue(
		t.Context(),
		"send_email",
		args,
		opts,
	)

	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	if id == 0 {
		t.Fatalf("Enqueue returned id %d", id)
	}

	var (
		queue       string
		maxAttempts int
		runAt       time.Time
	)

	err = db.QueryRow(
		t.Context(),
		"SELECT queue, max_attempts, run_at FROM jobs WHERE id = $1",
		id,
	).Scan(&queue, &maxAttempts, &runAt)

	if err != nil {
		t.Fatalf("DB query failed: %v", err)
	}

	if queue != "default" {
		t.Errorf("queue = %q, want default", queue)
	}

	if maxAttempts != 20 {
		t.Errorf("maxAttempts = %d, want 20", maxAttempts)
	}

	if duration := time.Since(runAt); duration > 5*time.Second || duration < -5*time.Second {
		t.Errorf("run_at = %v, want within 5s of now", runAt)
	}
}

func TestEnqueue_WithQueue(t *testing.T) {
	opts := &jobq.EnqueueOpts{
		Queue: "emails",
	}

	client, db := newTestClient(t)

	args := emailArgs{
		To: "user@example.com",
	}

	id, err := client.Enqueue(t.Context(), "send_email", args, opts)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	if id == 0 {
		t.Fatalf("Enqueue returned id %d", id)
	}

	var queue string

	err = db.QueryRow(
		t.Context(),
		"SELECT queue FROM jobs WHERE id = $1",
		id,
	).Scan(&queue)

	if err != nil {
		t.Fatalf("DB query failed: %v", err)
	}

	if queue != "emails" {
		t.Fatalf("queue = %q, want %q", queue, "emails")
	}
}

func TestEnqueue_Defaults(t *testing.T) {
	client, db := newTestClient(t)

	args := emailArgs{
		To: "user@example.com",
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
	client, db := newTestClient(t)

	args := emailArgs{
		To: "user@example.com",
	}
	id, err := client.Enqueue(t.Context(), "", args, nil)
	if !errors.Is(err, jobq.ErrEmptyKind) {
		t.Fatalf("got error %v, want %v", err, jobq.ErrEmptyKind)
	}

	if id != 0 {
		t.Errorf("id should be 0")
	}

	count := countJobs(t, db)
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
			client, db := newTestClient(t)
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
