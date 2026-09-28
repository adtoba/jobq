package jobq

import (
	"context"
	"time"
)

type EnqueueOpts struct {
	Queue       string // nil value: "default"
	Priority    int16
	RunAt       time.Time // nil value: "run now"
	MaxAttempts int       // zero value: "use default (1)"
}

func (c *Client) Enqueue(ctx context.Context, kind string, args any, opts *EnqueueOpts) (int64, error) {
	return 0, nil
}
