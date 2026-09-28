package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/adtoba/jobq/internal/testdb"
	"github.com/adtoba/jobq/store"
)

func TestMigrate_AppliesAndIsIdempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testdb.NewRaw(t)

	for i := 0; i < 2; i++ {
		if err := store.Migrate(ctx, pool); err != nil {
			t.Fatalf("migrate run %d: %v", i+1, err)
		}
	}

	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM jobq_migrations").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("want 1 recorded migration, got %d", n)
	}

	var exists bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('public.jobs') IS NOT NULL").Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("jobs table was not created")
	}
}

// Several processes booting at once must not trip over each other.
func TestMigrate_ConcurrentCallersAreSafe(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testdb.NewRaw(t)

	const callers = 8
	var wg sync.WaitGroup
	errs := make(chan error, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- store.Migrate(ctx, pool)
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent migrate: %v", err)
		}
	}
}

func TestSchema_Defaults(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testdb.New(t)

	var (
		queue, state      string
		attempt, maxAttem int
	)
	err := pool.QueryRow(ctx,
		`INSERT INTO jobs (kind) VALUES ('noop') RETURNING queue, state, attempt, max_attempts`,
	).Scan(&queue, &state, &attempt, &maxAttem)
	if err != nil {
		t.Fatal(err)
	}
	if queue != "default" || state != "available" || attempt != 0 || maxAttem != 20 {
		t.Fatalf("unexpected defaults: queue=%q state=%q attempt=%d max_attempts=%d",
			queue, state, attempt, maxAttem)
	}
}

func TestSchema_RejectsUnknownState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testdb.New(t)

	_, err := pool.Exec(ctx, `INSERT INTO jobs (kind, state) VALUES ('noop', 'bogus')`)
	assertPgCode(t, err, "23514") // check_violation
}

func TestSchema_UniqueKeyOnlyBlocksLiveJobs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testdb.New(t)

	if _, err := pool.Exec(ctx, `INSERT INTO jobs (kind, unique_key) VALUES ('report', 'daily:2026-09-27')`); err != nil {
		t.Fatal(err)
	}

	// A second live job with the same key is rejected.
	_, err := pool.Exec(ctx, `INSERT INTO jobs (kind, unique_key) VALUES ('report', 'daily:2026-09-27')`)
	assertPgCode(t, err, "23505") // unique_violation

	// Once the first one finishes, the key is free again.
	if _, err := pool.Exec(ctx, `UPDATE jobs SET state = 'completed' WHERE unique_key = 'daily:2026-09-27'`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO jobs (kind, unique_key) VALUES ('report', 'daily:2026-09-27')`); err != nil {
		t.Fatalf("key should be reusable after completion: %v", err)
	}
}

func assertPgCode(t *testing.T, err error, code string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("want Postgres error %s, got %v", code, err)
	}
	if pgErr.Code != code {
		t.Fatalf("want Postgres error %s, got %s (%s)", code, pgErr.Code, pgErr.Message)
	}
}
