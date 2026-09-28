package jobq

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	ErrEmptyKind          = errors.New("jobq: kind is required")
	ErrEmptyArgs          = errors.New("jobq: args is required")
	ErrInvalidMaxAttempts = errors.New("jobq: invalid max attempts value")
)

const (
	defaultQueue       = "default"
	defaultMaxAttempts = 20
)

type EnqueueOpts struct {
	Queue       string    // zero value: "default"
	Priority    int16     // zero value: 0
	RunAt       time.Time // zero value: "run now"
	MaxAttempts int       // zero value: 20 (defaultMaxAttempts)
}

func withDefaults(opts *EnqueueOpts) EnqueueOpts {
	out := EnqueueOpts{}

	if opts != nil {
		out = *opts
	}

	if out.Queue == "" {
		out.Queue = defaultQueue
	}

	if out.MaxAttempts == 0 {
		out.MaxAttempts = defaultMaxAttempts
	}

	return out
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

	isNull := bytes.Equal(argsJSON, []byte("null"))
	if isNull {
		return 0, ErrEmptyArgs
	}

	o := withDefaults(opts)

	if o.MaxAttempts < 0 {
		return 0, fmt.Errorf("%w: got %d", ErrInvalidMaxAttempts, o.MaxAttempts)
	}

	var id int64
	var runAt *time.Time

	if !o.RunAt.IsZero() {
		runAt = &o.RunAt
	}

	err = c.pool.QueryRow(
		ctx,
		"INSERT INTO jobs (queue, kind, args, priority, max_attempts, run_at) VALUES ($1, $2, $3, $4, $5, COALESCE($6, now())) RETURNING id",
		o.Queue,
		kind,
		argsJSON,
		o.Priority,
		o.MaxAttempts,
		runAt,
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("jobq: insert job: %w", err)
	}

	return id, nil
}
