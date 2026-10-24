//go:generate go run ./internal/generate/main.go

package inference

import (
	"context"
	_ "embed"
	"time"
)

// diagnosisSchemaBytes contains the JSON schema for the DiagnosisResult structure.
// This allows us to embed the schema directly into the Go binary instead of reading the file at runtime.
// This is used by openai and gemini clients to tell their model api to enforce the structured output format.
//
//go:embed diagnosis_result.json
var diagnosisSchemaBytes []byte

type ProviderType string

const (
	ProviderGemini ProviderType = "gemini"
	ProviderOpenAI ProviderType = "openai"
)

// ThinkingLevel is the reasoning intensity requested from the model. The zero value disables thinking.
type ThinkingLevel string

const (
	ThinkingDisabled ThinkingLevel = ""
	ThinkingMinimal  ThinkingLevel = "minimal"
	ThinkingLow      ThinkingLevel = "low"
	ThinkingMedium   ThinkingLevel = "medium"
	ThinkingHigh     ThinkingLevel = "high"
)

// InferenceConfig dictates which provider and model to route the request to, and how to run it.
//
// The sampling options are pointers so that an explicit 0 can be told apart from "not set"; a nil
// option is left out of the request and the provider's default applies.
type InferenceConfig struct {
	Provider ProviderType // e.g., ProviderGemini or ProviderOpenAI
	Model    string       // e.g., "gemini-3.1-flash-lite" or "gpt-4o-mini"

	Temperature *float32
	TopP        *float32
	// TopK is ignored by providers that do not support it (OpenAI).
	TopK *int32

	Thinking        ThinkingLevel
	IncludeThoughts bool // Whether to include the model's intermediate reasoning steps in the output

	// MaxTokens caps generated tokens; 0 uses the model default. OpenAI includes reasoning tokens.
	// Both providers reject negative values; Gemini also rejects values above math.MaxInt32.
	MaxTokens int

	// Optional fallback model if the primary provider hits a rate limit
	Fallback *InferenceConfig
}

// This is the minimum set of fields that we expect the log integration to provide.
type LogEntry struct {
	// unique identifier for the log entry, generated at Ingestion time
	ID                 string    `json:"id"`
	IngestionTimestamp time.Time `json:"ingestion_timestamp"`
	LogSourceID        uint16    `json:"log_source_id"` // id for the log source integration, unique per source and enforced by the consuming codebase

	// These fields are extracted from the log source (from which ever external integration the log originated)

	Timestamp  string         `json:"timestamp"`
	Message    string         `json:"message"`
	Service    string         `json:"service,omitempty"`
	Source     string         `json:"source,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"` // catchall for all other fields
}

// LogRetriever allows the inference engine to fetch logs based on a given query.
type LogRetriever interface {
	Query(ctx context.Context, query string) ([]LogEntry, error)
}

// LLMClient represents a specific provider's implementation (e.g., GeminiClient).
type LLMClient interface {
	// Ready validates provider configuration and performs a bounded provider readiness request.
	Ready(ctx context.Context) error

	// Analyze consumes the system instruction and the raw telemetry context,
	// returning the response unmarshaled into our strict struct.
	Analyze(ctx context.Context, cfg InferenceConfig, systemPrompt string, logContext []LogInput) (DiagnosisResult, error)

	// Close gracefully shuts down underlying HTTP or gRPC connections.
	Close() error
}

// LLMFactory acts as a router, instantiating or returning the correct client
// based on the incoming InferenceConfig.
type LLMFactory interface {
	GetClient(ctx context.Context, provider ProviderType) (LLMClient, error)
}

// Engine orchestrates the root-cause analysis pipeline.
type Engine interface {
	// Diagnose is the primary entrypoint. It takes a LogsQL query, fetches the
	// corresponding telemetry, and requests an analysis from the LLM.
	Diagnose(ctx context.Context, logsQuery string, cfg InferenceConfig) (DiagnosisResult, error)

	// DiagnoseWithContext allows bypassing the LogRetriever if the anomalous
	// logs were already intercepted in real-time (e.g., via a streaming WAL).
	DiagnoseWithContext(ctx context.Context, anomalousLogs []LogInput, cfg InferenceConfig) (DiagnosisResult, error)
}
