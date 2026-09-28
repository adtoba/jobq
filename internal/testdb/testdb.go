// Package testdb gives each test its own throwaway Postgres database, so tests
// can run in parallel without seeing each other's jobs.
package testdb

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/adtoba/jobq/store"
)

const defaultURL = "postgres://jobq:jobq@localhost:5432/jobq?sslmode=disable"

var counter atomic.Int64

func baseURL() string {
	if u := os.Getenv("DATABASE_URL"); u != "" {
		return u
	}
	return defaultURL
}

// New returns a pool connected to a fresh database with all migrations applied.
func New(t testing.TB) *pgxpool.Pool {
	t.Helper()
	pool := NewRaw(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := store.Migrate(ctx, pool); err != nil {
		t.Fatalf("testdb: migrate: %v", err)
	}
	return pool
}

// NewRaw returns a pool connected to a fresh, empty database (no migrations).
// The database is dropped when the test finishes.
func NewRaw(t testing.TB) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	admin, err := pgx.Connect(ctx, baseURL())
	if err != nil {
		t.Fatalf("testdb: connect to %s: %v\n(is Postgres running? try `make db-up`)", baseURL(), err)
	}

	name := fmt.Sprintf("jobq_test_%d_%d_%d", os.Getpid(), time.Now().UnixNano(), counter.Add(1))
	ident := pgx.Identifier{name}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+ident); err != nil {
		admin.Close(ctx)
		t.Fatalf("testdb: create database: %v", err)
	}

	cfg, err := pgxpool.ParseConfig(baseURL())
	if err != nil {
		admin.Close(ctx)
		t.Fatalf("testdb: parse url: %v", err)
	}
	cfg.ConnConfig.Database = name

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		admin.Close(ctx)
		t.Fatalf("testdb: open pool: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		cctx, ccancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer ccancel()
		if _, err := admin.Exec(cctx, "DROP DATABASE IF EXISTS "+ident+" WITH (FORCE)"); err != nil {
			t.Errorf("testdb: drop database %s: %v", name, err)
		}
		admin.Close(cctx)
	})
	return pool
}
