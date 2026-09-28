package jobq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	ErrEmptyKind = errors.New("jobq: kind is required")
	ErrEmptyArgs = errors.New("jobq: args is required")
)

type EnqueueOpts struct {
	Queue       string // nil value: "default"
	Priority    int16
	RunAt       time.Time // nil value: "run now"
	MaxAttempts int       // zero value: "use default (1)"
}

func (c *Client) Enqueue(ctx context.Context, kind string, args any, opts *EnqueueOpts) (int64, error) {
	if kind == "" {
		return 0, ErrEmptyKind
	}

	if args == nil {
		return 0, ErrEmptyArgs
	}

	argsJSON, err := json.Marshal(args)
	if err != nil {
		return 0, fmt.Errorf("jobq: invalid args: %w", err)
	}

	var id int64

	err = c.pool.QueryRow(
		ctx,
		"INSERT INTO jobs (kind, args) VALUES ($1, $2) RETURNING id",
		kind,
		argsJSON,
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("jobq: insert job: %w", err)
	}

	return id, nil
}
