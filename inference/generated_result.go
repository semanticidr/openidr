// Code generated from JSON Schema using quicktype. DO NOT EDIT.
// To parse and unparse this JSON data, add this code to your project and do:
//
//    diagnosisResult, err := UnmarshalDiagnosisResult(bytes)
//    bytes, err = diagnosisResult.Marshal()

package inference

import "encoding/json"

func UnmarshalDiagnosisResult(data []byte) (DiagnosisResult, error) {
	var r DiagnosisResult
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *DiagnosisResult) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type DiagnosisResult struct {
	// ISO 8601 timestamp of incident start. Use the earliest relevant log timestamp.
	OccurredAt string `json:"occurredAt"`
	// Secondary technical factors or misconfigurations that worsened the failure.
	PossibleContributingFactors []string `json:"possibleContributingFactors"`
	// Prioritized list of next steps.
	RecommendedActions []RecommendedAction `json:"recommendedActions"`
	// Why this happened, single definitive sentence identifying the underlying technical cause.
	// State 'Unknown' if unsupported by evidence.
	RootCause string `json:"rootCause"`
	// Incident impact: 0(Critical/Breach), 1(Major), 2(Degraded), 3(Minor), 4(Noise)
	Severity int64 `json:"severity"`
	// What happened, 2-4 sentences detailing the failure mechanism and its blast radius.
	Summary string `json:"summary"`
	// Chronological sequence of critical state changes.
	Timeline []Timeline `json:"timeline"`
	// Concise technical title of the incident.
	Title string `json:"title"`
}

type RecommendedAction struct {
	// Specific, actionable engineering task.
	Action string `json:"action"`
	Type   Type   `json:"type"`
}

type Timeline struct {
	// Factual description of the state change or error.
	Event string `json:"event"`
	// Associated logIds (as provided in input).
	LogIDS []string `json:"logIds"`
}

type Type string

const (
	Communication Type = "communication"
	Investigation Type = "investigation"
	Mitigation    Type = "mitigation"
	Other         Type = "other"
	Prevention    Type = "prevention"
)
