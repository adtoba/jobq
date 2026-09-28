package jobq

import (
	"context"
	"errors"
)

var (
	ErrNoJobs = errors.New("jobq: No jobs available")
)

func (c *Client) claim(ctx context.Context, queue string, workerID string) (*Job, error) {
	return nil, nil
}
