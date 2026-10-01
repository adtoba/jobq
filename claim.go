package jobq

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

var (
	errNoJobs = errors.New("jobq: no jobs available")
)

const claimQuery = `
	UPDATE jobs SET state = 'running',
		attempt = attempt + 1,
		locked_by = $2
	WHERE id = (
		SELECT id
		FROM jobs
		WHERE queue = $1
			AND state = 'available'
			AND run_at <= now()
		ORDER BY priority DESC, run_at, id
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	)
	RETURNING id, queue, kind, args, attempt, max_attempts
`

const claimBatchQuery = `
	UPDATE jobs SET state = 'running',
		attempt = attempt + 1,
		locked_by = $2
	WHERE id IN (
		SELECT id
		FROM jobs
		WHERE queue = $1
			AND state = 'available'
			AND run_at <= now()
		ORDER BY priority DESC, run_at, id
		LIMIT $3
		FOR UPDATE SKIP LOCKED
	)
	RETURNING id, queue, kind, args, attempt, max_attempts
`

func (c *Client) claim(ctx context.Context, queue string, workerID string) (*Job, error) {
	var job Job

	err := c.pool.QueryRow(ctx, claimQuery, queue, workerID).Scan(
		&job.ID, &job.Queue, &job.Kind, &job.Args, &job.Attempt, &job.MaxAttempts,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errNoJobs
		}
		return nil, fmt.Errorf("jobq: claim job: %w", err)
	}

	return &job, nil
}

func (c *Client) claimBatch(ctx context.Context, queue, workerID string, limit int) ([]*Job, error) {
	rows, err := c.pool.Query(ctx, claimBatchQuery, queue, workerID, limit)
	if err != nil {
		return nil, fmt.Errorf("jobq: DB query failed: %w", err)
	}
	defer rows.Close()

	var jobs []*Job
	for rows.Next() {
		var job Job
		if err = rows.Scan(&job.ID, &job.Queue, &job.Kind, &job.Args, &job.Attempt, &job.MaxAttempts); err != nil {
			return nil, fmt.Errorf("jobq: scan job: %w", err)
		}
		jobs = append(jobs, &job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("jobq: list jobs: %w", err)
	}

	return jobs, nil
}
