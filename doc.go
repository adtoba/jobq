// Package jobq is a Postgres-backed background job queue.
//
// Delivery is at-least-once: every job runs to completion or is discarded
// after its final attempt, but after a crash a job may run more than once.
// Handlers must therefore be idempotent.
package jobq
