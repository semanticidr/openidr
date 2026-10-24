package inference

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/semanticidr/openidr/inference/internal/diagnosisschema"
)

// loadDiagnosisSchema loads and validates the diagnosis schema document, returning the parsed schema and its raw JSON bytes.
func loadDiagnosisSchema() (diagnosisschema.GeneratedDiagnosis, []byte, error) {
	document, err := diagnosisschema.UnmarshalGeneratedDiagnosis(diagnosisSchemaBytes)
	if err != nil {
		return document, nil, fmt.Errorf("decode diagnosis schema document: %w", err)
	}
	// Preserve the original schema, including keywords not modeled by the generated types.
	var raw struct {
		Schema json.RawMessage `json:"schema"`
	}
	if err := json.Unmarshal(diagnosisSchemaBytes, &raw); err != nil {
		return document, nil, err
	}
	if document.Name == "" || len(raw.Schema) == 0 {
		return document, nil, fmt.Errorf("diagnosis schema document is missing its name or schema")
	}
	return document, raw.Schema, nil
}

// ParseDiagnosisResult decodes raw LLM output and then applies application-level validation.
func ParseDiagnosisResult(data []byte) (DiagnosisResult, error) {
	result, err := UnmarshalDiagnosisResult(data)
	if err != nil {
		return DiagnosisResult{}, fmt.Errorf("%w: decode diagnosis result: %w", ErrInvalidOutput, err)
	}
	if err := result.Validate(); err != nil {
		return DiagnosisResult{}, fmt.Errorf("%w: %w", ErrInvalidOutput, err)
	}
	return result, nil
}

// Validate checks application-level invariants on a decoded diagnosis: the fields the incident
// record needs are present, the severity scores are on their scales, the timestamp parses and every
// action has a known type.
func (r *DiagnosisResult) Validate() error {
	if r.Title == "" {
		return fmt.Errorf("title is required")
	}
	if r.Summary == "" {
		return fmt.Errorf("summary is required")
	}
	if r.RootCause == "" {
		return fmt.Errorf("rootCause is required")
	}
	if r.Severity < 0 || r.Severity > 4 {
		return fmt.Errorf("severity %d is outside 0-4", r.Severity)
	}
	if _, err := time.Parse(time.RFC3339Nano, r.OccurredAt); err != nil {
		return fmt.Errorf("occurredAt %q is not an RFC 3339 timestamp", r.OccurredAt)
	}
	for _, action := range r.RecommendedActions {
		switch action.Type {
		case Communication, Investigation, Mitigation, Other, Prevention:
		default:
			return fmt.Errorf("unknown action type %q", action.Type)
		}
	}
	return nil
}

// DropUnknownLogs removes references to logs that were not part of the input. A model can cite IDs
// it was never given, and an incident must not point at logs that do not exist. Timeline events are
// kept even when none of their logs are known.
func (r *DiagnosisResult) DropUnknownLogs(inputs []LogInput) {
	known := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		known[input.ID] = struct{}{}
	}

	for i := range r.Timeline {
		ids := r.Timeline[i].LogIDS[:0]
		for _, id := range r.Timeline[i].LogIDS {
			if _, ok := known[id]; ok {
				ids = append(ids, id)
			}
		}
		r.Timeline[i].LogIDS = ids
	}
}
