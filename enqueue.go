package jobq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type EnqueueOpts struct {
	Queue       string // nil value: "default"
	Priority    int16
	RunAt       time.Time // nil value: "run now"
	MaxAttempts int       // zero value: "use default (1)"
}

func (c *Client) Enqueue(ctx context.Context, kind string, args any, opts *EnqueueOpts) (int64, error) {
	if kind == "" {
		return 0, errors.New("jobq: kind cannot be null")
	}
	_, err := json.Marshal(&args)
	if err != nil {
		return 0, fmt.Errorf("jobq: Invalid args: %w", err)
	}

	var id int64

	err = c.pool.QueryRow(
		ctx,
		"INSERT INTO jobs (kind, args) VALUES ($1, $2) RETURNING id",
		kind,
		args,
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("jobq: Error inserting a new job: %w", err)
	}

	return id, nil
}
