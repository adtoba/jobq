# jobq design notes

A running record of the decisions behind jobq, why they were made, and what
the experiments showed. Newest sections go at the bottom.

## Delivery guarantee: at-least-once

Every job either completes or is discarded after its final attempt. But a job
can run more than once: if a worker finishes a job and crashes before recording
"done", another worker will run it again. No queue can prevent this on its own,
so **handlers must be idempotent** (safe to run twice).

## Storage

Postgres is the only dependency. Jobs are rows in a `jobs` table, and a job's
life is a state machine:

```
available ──► running ──► completed
                 │
                 └──────► discarded   (out of attempts)
```

Scheduled and retrying jobs are just `available` rows whose `run_at` is in the
future, which keeps the fetch query and its index simple.

Migrations are embedded in the binary and applied by `store.Migrate`, which
takes a Postgres advisory lock first so several processes starting at once
apply them one at a time. Without the lock, even `CREATE TABLE IF NOT EXISTS`
fails when run concurrently.

## Client

`Client` wraps a `*pgxpool.Pool`. The pool field is unexported so users can't
swap it after construction (which could split jobs across two databases), and
so the internals can change without breaking callers.

## Enqueue

`Enqueue(ctx, kind, args, opts)` inserts one job and returns its ID.

**Validation happens in Go, before the database.** An empty kind returns
`ErrEmptyKind`. Postgres wouldn't catch it: `NOT NULL` only blocks SQL `NULL`,
and an empty string isn't `NULL`. Errors are package-level sentinels so callers
can check them with `errors.Is`, even if they're later wrapped with more context.

**Nil args and the typed-nil trap.** An interface holds a type and a value, and
is only `== nil` when both are empty. A nil `*emailArgs` passed as `any` is
"type `*emailArgs`, value nil", so `args == nil` is false and it slips through.
Instead of checking the input, jobq checks the output: every form of nil
marshals to the JSON `null`, so `bytes.Equal(argsJSON, []byte("null"))` catches
all of them and returns `ErrEmptyArgs`.

Empty-but-real args like `emailArgs{}` are accepted. Whether an empty email
address is valid is the handler's business, not the queue's.

**Options follow "make the zero value useful".** Every zero value in
`EnqueueOpts` safely means "use the default": nobody means an empty queue name,
a priority of 0 is the default, nobody schedules a job for the year 1, and
0 max attempts isn't a real request. So `nil` opts and `&EnqueueOpts{}` mean the
same thing.

**Defaults are applied per field, on a copy.** `withDefaults` copies the
caller's struct and fills in only the zero fields. Defaults apply per field
(a caller who sets `Priority` still expects the default queue), and working on
a copy means the caller's struct is never modified behind their back.

**`run_at` uses the database clock.** When `RunAt` is unset, jobq sends SQL
`NULL` (a nil `*time.Time`) and the query uses `COALESCE($6, now())`. Passing
the zero `time.Time` directly would store the year 1, because `COALESCE` only
replaces `NULL`, and a zero time isn't `NULL`. Using Postgres's `now()` rather
than Go's `time.Now()` means every job's time comes from one clock, instead of
from several app servers whose clocks may disagree.

## Claiming a job

A worker claims a job by marking it `running` so no other worker takes it.

**It must be a single statement.** If claiming were a `SELECT` followed by an
`UPDATE`, two workers could both see job 7 as available, both mark it running,
and both run it: the customer gets the welcome email twice. So the claim is one
`UPDATE ... WHERE id = (SELECT ... FOR UPDATE SKIP LOCKED) RETURNING ...`:

- The inner `SELECT` picks the best job: this queue, `available`, due now,
  highest priority first, then oldest, with `id` as a tie-breaker.
- `FOR UPDATE` locks that row. `SKIP LOCKED` makes other workers skip rows
  that are already locked instead of waiting for them.
- The outer `UPDATE` marks it `running`, increments `attempt`, and records the
  worker in `locked_by`. `RETURNING` hands back the job in the same statement.

**The database lock is short.** It only lasts for the claim itself. After that,
`state = 'running'` keeps other workers away, since they only look at
`available` jobs. The handler runs with no transaction open, so a slow handler
doesn't hold a connection or a row lock.

**Empty queue.** When nothing qualifies, the `UPDATE` returns no row and pgx
returns `pgx.ErrNoRows`, which `claim` turns into `errNoJobs`. It's unexported
because only jobq's own worker ever sees it. It lets the worker tell "queue is
empty" apart from "database is down".

### Experiment: removing the lock

20 workers claiming at the same moment, 20 runs each:

| Claim query                 | Result                                              |
|-----------------------------|-----------------------------------------------------|
| No locking                  | Double-claimed in every run: 2–4 workers "won" the same job, `attempt` reached 4 |
| `FOR UPDATE` only           | Correct: exactly one winner. With 20 jobs, every worker got one |
| `FOR UPDATE SKIP LOCKED`    | Correct: exactly one winner                          |

`FOR UPDATE` alone is correct: a worker that waits on a locked row finds it no
longer matches when the lock is released, and Postgres moves on to the next row.
What `SKIP LOCKED` adds is not waiting. Without it, workers line up behind each
other's locks one at a time; with it, they take different rows in parallel. The
difference is small when claims are fast, and grows when locks are held longer,
such as when claiming batches of jobs.

### Lesson: a passing concurrency test may not be testing anything

The first version of the concurrency test passed even with the lock removed.
The goroutines were released together, but each then had to open its own
database connection (pgx opens them lazily), and that took several
milliseconds. The first worker claimed the job before the others even reached
the database, so they never actually overlapped.

The fix was to warm up every connection before opening the gate: each goroutine
runs `SELECT 1`, signals a second `WaitGroup`, and only then waits for the gate.
With that change, the no-lock query failed in every run and the locked query
passed in every run.

Takeaways:
- A test that still passes after you remove the protection isn't testing the
  protection. Break things on purpose to prove the test can fail.
- At most as many claims can run truly in parallel as the pool has connections.

## Testing approach

- Each test gets its own throwaway database, so tests run in parallel without
  seeing each other's jobs.
- Tests are written first and must be seen failing before the code is written.
- Error paths get their own tests, checked with `errors.Is`.
- Everything runs under the race detector (`go test -race`), and concurrency
  tests are repeated with `-count=20`.

## Open items

- Tests for `Priority` and `MaxAttempts` options, and for negative
  `MaxAttempts` returning `ErrInvalidMaxAttempts`.
- `EnqueueOpts` field comments should say "zero value", not "nil value".
