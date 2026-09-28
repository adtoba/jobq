# jobq

A Postgres-backed background job queue for Go, built step by step to learn
Go concurrency and distributed-systems design.

**Delivery guarantee: at-least-once.** Every job either completes or is
discarded after its final attempt, but after a crash a job can run more than
once. Handlers must be idempotent.

## Development

Requires Go 1.22+ and Docker.

```sh
make db-up       # start Postgres
make check       # go vet + tests with the race detector (what CI runs)
make psql        # poke around the database
make db-reset    # start over with an empty database
```

Each test gets its own throwaway database (see `internal/testdb`), so tests
run in parallel without interfering with each other.

## Layout

```
doc.go                 package jobq: the public library (grows from step 1)
store/                 schema, migrations, SQL
store/migrations/      NNN_description.sql, embedded into the binary
internal/testdb/       per-test databases
```

## Roadmap

- [x] 0. Setup: repo, Postgres, migrations, CI
- [ ] 1. Simplest loop: enqueue, fetch, run, complete
- [ ] 2. Concurrency: executor pool, backpressure, typed handlers
- [ ] 3. Failure handling: retries, backoff, discard
- [ ] 4. Crash safety: leases, heartbeats, fencing, rescuer
- [ ] 5. Graceful shutdown
- [ ] 6. Multiple processes + LISTEN/NOTIFY
- [ ] 7. Leader election, cron, unique jobs
- [ ] 8. Transactional enqueue
- [ ] 9. Metrics, admin CLI, cleanup
- [ ] 10. Load test, profiling, write-up
