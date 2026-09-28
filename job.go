package jobq

import "encoding/json"

type Job struct {
	ID          int64
	Queue       string
	Kind        string
	Args        json.RawMessage
	Attempt     int
	MaxAttempts int
}
