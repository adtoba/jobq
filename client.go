package jobq

import (
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNilPool = errors.New("jobq: NewClient called with nil")

type Client struct {
	pool *pgxpool.Pool
}

func NewClient(pool *pgxpool.Pool) (*Client, error) {
	if pool != nil {
		client := &Client{
			pool: pool,
		}
		return client, nil
	}
	return nil, ErrNilPool
}
