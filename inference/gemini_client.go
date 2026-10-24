// gemini_client.go
package inference

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"

	"google.golang.org/genai"
)

type GeminiClient struct {
	client       *genai.Client
	geminiSchema *genai.Schema
}

func NewGeminiClientFromEnv(ctx context.Context) (*GeminiClient, error) {
	switch os.Getenv("GEMINI_AUTH_MODE") {
	case "api_key":
		apiKey := os.Getenv("GEMINI_API_KEY")
		if strings.TrimSpace(apiKey) == "" {
			return nil, fmt.Errorf("GEMINI_API_KEY must be set when GEMINI_AUTH_MODE=api_key")
		}
		return NewGeminiAPIKeyClient(ctx, apiKey)
	case "vertex":
		projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
		location := os.Getenv("GOOGLE_CLOUD_LOCATION")
		if strings.TrimSpace(projectID) == "" || strings.TrimSpace(location) == "" {
			return nil, fmt.Errorf("GOOGLE_CLOUD_PROJECT and GOOGLE_CLOUD_LOCATION must be set when GEMINI_AUTH_MODE=vertex")
		}
		return NewGeminiClient(ctx, projectID, location)
	default:
		return nil, fmt.Errorf("GEMINI_AUTH_MODE must be set to api_key or vertex")
	}
}

func NewGeminiClient(ctx context.Context, projectID, location string) (*GeminiClient, error) {
	config := &genai.ClientConfig{
		Backend:  genai.BackendVertexAI,
		Project:  projectID,
		Location: location,
	}

	client, err := genai.NewClient(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize genai client: %w", err)
	}
	return newGeminiClient(client)
}

// NewGeminiAPIKeyClient connects to the Gemini API with an API key.
func NewGeminiAPIKeyClient(ctx context.Context, apiKey string) (*GeminiClient, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{Backend: genai.BackendGeminiAPI, APIKey: apiKey})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize genai client: %w", err)
	}
	return newGeminiClient(client)
}

func newGeminiClient(client *genai.Client) (*GeminiClient, error) {
	_, innerSchemaBytes, err := loadDiagnosisSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to parse embedded schema wrapper: %w", err)
	}
	var schema genai.Schema
	if err := json.Unmarshal(innerSchemaBytes, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse inner schema for Gemini: %w", err)
	}

	return &GeminiClient{
		client:       client,
		geminiSchema: &schema,
	}, nil
}

func (c *GeminiClient) Analyze(ctx context.Context, cfg InferenceConfig, systemPrompt string, logContext []LogInput) (DiagnosisResult, error) {
	if cfg.MaxTokens < 0 || int64(cfg.MaxTokens) > math.MaxInt32 {
		return DiagnosisResult{}, fmt.Errorf("%w: Gemini MaxTokens must be between 0 and %d; 0 uses the model default", ErrBadRequest, math.MaxInt32)
	}
	config := c.generateConfig(cfg, systemPrompt)

	logsJSON, err := json.Marshal(logContext)
	if err != nil {
		return DiagnosisResult{}, fmt.Errorf("failed to marshal log context: %w", err)
	}

	resp, err := c.client.Models.GenerateContent(ctx, cfg.Model, genai.Text(string(logsJSON)), config)
	if err != nil {
		return DiagnosisResult{}, fmt.Errorf("gemini inference failed: %w", classifyProviderError(err))
	}

	for _, candidate := range resp.Candidates {
		if candidate != nil && candidate.FinishReason == genai.FinishReasonMaxTokens {
			return DiagnosisResult{}, fmt.Errorf("%w: Gemini reached its output token limit (MaxTokens=%d; 0 uses the model default); increase MaxTokens or request a shorter response", ErrInvalidOutput, cfg.MaxTokens)
		}
	}

	resultText := resp.Text()
	if resultText == "" {
		return DiagnosisResult{}, fmt.Errorf("%w: empty response received from Gemini", ErrInvalidOutput)
	}

	result, err := ParseDiagnosisResult([]byte(resultText))
	if err != nil {
		return DiagnosisResult{}, fmt.Errorf("invalid Gemini response: %w", err)
	}

	return result, nil
}

func (c *GeminiClient) Ready(ctx context.Context) error {
	for _, err := range c.client.Models.All(ctx) {
		if err != nil {
			return fmt.Errorf("Gemini model-list readiness check failed: %w", classifyProviderError(err))
		}
		return nil
	}
	return nil
}

// generateConfig builds the request options. Options that are not set are left out so the model's
// defaults apply.
func (c *GeminiClient) generateConfig(cfg InferenceConfig, systemPrompt string) *genai.GenerateContentConfig {
	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: systemPrompt}},
		},
		ResponseMIMEType: "application/json",
		ResponseSchema:   c.geminiSchema,
		Temperature:      cfg.Temperature,
		TopP:             cfg.TopP,
	}
	if cfg.TopK != nil {
		config.TopK = new(float32(*cfg.TopK))
	}
	if cfg.MaxTokens > 0 {
		config.MaxOutputTokens = int32(cfg.MaxTokens)
	}

	if level, ok := geminiThinkingLevel(cfg.Thinking); ok {
		config.ThinkingConfig = &genai.ThinkingConfig{ThinkingLevel: level, IncludeThoughts: cfg.IncludeThoughts}
	}
	return config
}

func geminiThinkingLevel(level ThinkingLevel) (genai.ThinkingLevel, bool) {
	switch level {
	case ThinkingMinimal:
		return genai.ThinkingLevelMinimal, true
	case ThinkingLow:
		return genai.ThinkingLevelLow, true
	case ThinkingMedium:
		return genai.ThinkingLevelMedium, true
	case ThinkingHigh:
		return genai.ThinkingLevelHigh, true
	default:
		return "", false
	}
}

func (c *GeminiClient) Close() error {
	return nil
}
