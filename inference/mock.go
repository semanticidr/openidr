// inference_mock.go
package inference

import (
	"context"
	"fmt"
	"log"
	"time"
)

// MockLLMClient satisfies the LLMClient interface without making network calls.
type MockLLMClient struct{}

func (m *MockLLMClient) Ready(context.Context) error { return nil }

func (m *MockLLMClient) Analyze(ctx context.Context, cfg InferenceConfig, systemPrompt string, logContext []LogInput) (DiagnosisResult, error) {
	log.Printf("[MockLLMClient] Analyzing %d logs using %s model: %s", len(logContext), cfg.Provider, cfg.Model)

	// Simulate context cancellation or timeouts
	if err := ctx.Err(); err != nil {
		return DiagnosisResult{}, err
	}

	// Return a hardcoded structured response mimicking an LLM output
	return DiagnosisResult{
		OccurredAt:                  time.Now().UTC().Format(time.RFC3339),
		Title:                       fmt.Sprintf("Simulated Outage (Model: %s)", cfg.Model),
		Summary:                     "A simulated analysis based on the provided telemetry data.",
		RootCause:                   "The system encountered an artificial error injected for testing the LLM plumbing.",
		PossibleContributingFactors: []string{},
		Timeline:                    []Timeline{},
		Severity:                    3,
		RecommendedActions: []RecommendedAction{{
			Type:   Investigation,
			Action: "Verify orchestration plumbing and proceed to integrate the actual Google Gemini SDK.",
		}},
	}, nil
}

func (m *MockLLMClient) Close() error {
	log.Println("[MockLLMClient] Connections closed.")
	return nil
}

// SimpleLLMFactory provisions clients based on the configuration.
type SimpleLLMFactory struct {
	mockClient *MockLLMClient
}

func NewSimpleLLMFactory() *SimpleLLMFactory {
	return &SimpleLLMFactory{
		mockClient: &MockLLMClient{},
	}
}

func (f *SimpleLLMFactory) GetClient(_ context.Context, provider ProviderType) (LLMClient, error) {
	// In a real implementation, this would switch on 'provider' and instantiate
	// specific clients (e.g., GeminiClient, OpenAIClient).
	// For testing, we route everything to the mock.
	log.Printf("[SimpleLLMFactory] Provisioning client for provider: %s", provider)
	return f.mockClient, nil
}
