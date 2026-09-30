package jobq

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"
)

type HandlerFunc func(ctx context.Context, job *Job) error

type WorkerConfig struct {
	Queue        string
	ID           string
	PollInterval time.Duration
}

func withWorkerDefaults(config *WorkerConfig) WorkerConfig {
	out := WorkerConfig{}
	if config != nil {
		out = *config
	}
	if out.Queue == "" {
		out.Queue = defaultQueue
	}
	if out.PollInterval == 0 {
		out.PollInterval = defaultPollInterval
	}
	if out.ID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "worker"
		}
		pid := os.Getpid()
		out.ID = fmt.Sprintf("%s-%d", hostname, pid)
	}
	return out
}

type Worker struct {
	client       *Client
	queue        string
	workerID     string
	pollInterval time.Duration
	handlers     map[string]HandlerFunc
}

func NewWorker(client *Client, workerConfig *WorkerConfig) *Worker {
	config := withWorkerDefaults(workerConfig)

	return &Worker{
		client:       client,
		queue:        config.Queue,
		workerID:     config.ID,
		pollInterval: config.PollInterval,
		handlers:     make(map[string]HandlerFunc),
	}
}

func (w *Worker) Register(kind string, handler HandlerFunc) error {
	if kind == "" {
		return errors.New("jobq: kind not specified")
	}
	if handler == nil {
		return errors.New("jobq: handler function is required")
	}
	_, exists := w.handlers[kind]
	if exists {
		return fmt.Errorf("jobq: handler already registered for kind %q", kind)
	}

	w.handlers[kind] = handler

	return nil
}
