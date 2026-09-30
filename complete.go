package jobq

import (
	"context"
	"errors"
	"fmt"
)

var (
	errJobNotOwned = errors.New("jobq: job not owned")
)

const completeQuery = `
	UPDATE jobs SET state = 'completed',
		finalized_at = now()
	WHERE id = $1
	AND locked_by = $2
	AND state = 'running'
`

const failQuery = `
	UPDATE jobs 
	SET state = 'discarded',
		finalized_at = now(),
		errors = errors || jsonb_build_array(
			jsonb_build_object(
				'attempt', attempt,
				'at', now(),
				'error', $3::text
			)
		)
	WHERE id = $1
		AND locked_by = $2
		AND state = 'running'
`

func (c *Client) complete(ctx context.Context, jobID int64, workerID string) error {
	commandTag, err := c.pool.Exec(ctx, completeQuery, jobID, workerID)
	if err != nil {
		return fmt.Errorf("complete job: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return errJobNotOwned
	}

	return nil
}

func (c *Client) fail(ctx context.Context, jobID int64, workerID string, jobErr error) error {
	if jobErr == nil {
		jobErr = errors.New("unknown error")
	}
	res, err := c.pool.Exec(ctx, failQuery, jobID, workerID, jobErr.Error())
	if err != nil {
		return fmt.Errorf("jobq: fail job: %w", err)
	}
	if res.RowsAffected() == 0 {
		return errJobNotOwned
	}

	return nil
}
