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
