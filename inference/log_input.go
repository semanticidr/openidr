package inference

import (
	"errors"
	"fmt"
	"time"
)

// LogInput is a single log line sent to the LLM for analysis.
type LogInput struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	Service   string    `json:"service"`
}

// Validate reports whether all required fields are set.
func (l LogInput) Validate() error {
	var errs []error
	if l.ID == "" {
		errs = append(errs, errors.New("id is required"))
	}
	if l.Timestamp.IsZero() {
		errs = append(errs, errors.New("timestamp is required"))
	}
	if l.Message == "" {
		errs = append(errs, errors.New("message is required"))
	}
	if l.Service == "" {
		errs = append(errs, errors.New("service is required"))
	}
	return errors.Join(errs...)
}

// LogInputFromEntry converts a stored log entry into an LLM input.
func LogInputFromEntry(entry LogEntry) (LogInput, error) {
	ts, err := time.Parse(time.RFC3339Nano, entry.Timestamp)
	if err != nil {
		return LogInput{}, fmt.Errorf("log %q: parse timestamp %q: %w", entry.ID, entry.Timestamp, err)
	}
	input := LogInput{
		ID:        entry.ID,
		Timestamp: ts,
		Message:   entry.Message,
		Service:   entry.Service,
	}
	if err := input.Validate(); err != nil {
		return LogInput{}, fmt.Errorf("log %q: %w", entry.ID, err)
	}
	return input, nil
}

func validateLogInputs(inputs []LogInput) error {
	for i, input := range inputs {
		if err := input.Validate(); err != nil {
			return fmt.Errorf("log input %d: %w", i, err)
		}
	}
	return nil
}
