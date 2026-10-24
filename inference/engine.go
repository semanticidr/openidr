// engine.go
package inference

import (
	"context"
	"fmt"
	"log"
)

// RCAEngine implements the Engine interface.
type RCAEngine struct {
	retriever LogRetriever
	factory   LLMFactory
}

func NewRCAEngine(retriever LogRetriever, factory LLMFactory) *RCAEngine {
	return &RCAEngine{
		retriever: retriever,
		factory:   factory,
	}
}

func (e *RCAEngine) Diagnose(ctx context.Context, logsQuery string, cfg InferenceConfig) (DiagnosisResult, error) {
	log.Printf("[RCAEngine] Fetching logs for query: %s", logsQuery)

	// 1. Fetch context from the database abstraction
	entries, err := e.retriever.Query(ctx, logsQuery)
	if err != nil {
		return DiagnosisResult{}, fmt.Errorf("failed to retrieve logs: %w", err)
	}

	inputs := make([]LogInput, 0, len(entries))
	for _, entry := range entries {
		input, err := LogInputFromEntry(entry)
		if err != nil {
			return DiagnosisResult{}, fmt.Errorf("failed to prepare logs for inference: %w", err)
		}
		inputs = append(inputs, input)
	}

	if len(inputs) == 0 {
		log.Println("[RCAEngine] Warning: Query returned no logs. Proceeding with empty context.")
	}

	// 2. Hand off to the context-aware pipeline
	return e.DiagnoseWithContext(ctx, inputs, cfg)
}

func (e *RCAEngine) DiagnoseWithContext(ctx context.Context, anomalousLogs []LogInput, cfg InferenceConfig) (DiagnosisResult, error) {
	if err := validateLogInputs(anomalousLogs); err != nil {
		return DiagnosisResult{}, err
	}

	// 1. Provision the correct client
	client, err := e.factory.GetClient(ctx, cfg.Provider)
	if err != nil {
		return DiagnosisResult{}, fmt.Errorf("failed to get LLM client: %w", err)
	}

	// 2. Construct the system prompt instruction
	systemPrompt := `You are an expert site reliability engineer.
Analyze the provided JSON logs and determine the root cause of the incident.
Extract any affected pods, nodes, or services.
Format your response exactly matching the provided JSON schema.`

	// 3. Execute inference
	log.Printf("[RCAEngine] Triggering inference...")
	result, err := client.Analyze(ctx, cfg, systemPrompt, anomalousLogs)
	if err != nil {
		// 4. Graceful fallback handling
		if cfg.Fallback != nil {
			log.Printf("[RCAEngine] Primary provider failed: %v. Retrying with fallback model...", err)
			return e.DiagnoseWithContext(ctx, anomalousLogs, *cfg.Fallback)
		}
		return DiagnosisResult{}, fmt.Errorf("inference failed: %w", err)
	}

	// The clients already validate what they parse; this also covers any other LLMClient.
	if err := result.Validate(); err != nil {
		return DiagnosisResult{}, fmt.Errorf("%w: %w", ErrInvalidOutput, err)
	}
	result.DropUnknownLogs(anomalousLogs)
	return result, nil
}
