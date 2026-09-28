# jobq design notes

## Delivery guarantee
[What at-least-once means, and why handlers must be idempotent.]

## Enqueue
- Defaults: [Where they live and why. Why an empty EnqueueOpts means the same as nil.]
- Nil args: [The typed-nil trap, and why checking the JSON for `null` catches every case.]
- run_at: [Why an unset time is sent as NULL and filled by Postgres's now().]

## Claiming a job
Claiming a job must be a single statement. This is because if two workers are trying to claim a single job, it can execute the job multiple times if there is no lock structure in place.

### Experiment: removing the lock
[What happened with no lock, with FOR UPDATE only, and with FOR UPDATE SKIP LOCKED.
Include the numbers from your runs.]

### Lesson: a test that passes might not be testing anything
[Why the concurrency test passed without the lock at first, and how warming up
the connections fixed it.]