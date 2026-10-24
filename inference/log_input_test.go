package inference

import (
	"testing"
	"time"
)

func TestLogInputFromEntry(t *testing.T) {
	entry := LogEntry{ID: "log-1", Timestamp: "2026-10-02T10:00:00.5Z", Service: "api", Message: "boom"}
	input, err := LogInputFromEntry(entry)
	if err != nil {
		t.Fatal(err)
	}
	want := LogInput{ID: "log-1", Timestamp: time.Date(2026, 10, 2, 10, 0, 0, 5e8, time.UTC), Service: "api", Message: "boom"}
	if input != want {
		t.Fatalf("got %+v, want %+v", input, want)
	}

	for name, change := range map[string]func(*LogEntry){
		"missing id":        func(e *LogEntry) { e.ID = "" },
		"missing service":   func(e *LogEntry) { e.Service = "" },
		"missing message":   func(e *LogEntry) { e.Message = "" },
		"invalid timestamp": func(e *LogEntry) { e.Timestamp = "yesterday" },
	} {
		t.Run(name, func(t *testing.T) {
			bad := entry
			change(&bad)
			if _, err := LogInputFromEntry(bad); err == nil {
				t.Fatal("invalid entry accepted")
			}
		})
	}
}

func TestDiagnoseWithContextRejectsInvalidInput(t *testing.T) {
	engine := NewRCAEngine(nil, NewSimpleLLMFactory())
	_, err := engine.DiagnoseWithContext(t.Context(), []LogInput{{ID: "log-1"}}, InferenceConfig{})
	if err == nil {
		t.Fatal("invalid log input accepted")
	}
}
