// openai_client.go
package inference

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
)

type OpenAIClient struct {
	client     *openai.Client
	schemaMap  map[string]any
	schemaName string
	schemaDesc string
	strict     bool
}

func NewOpenAIClientFromEnv() (*OpenAIClient, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable is missing or empty")
	}
	return NewOpenAIClient(apiKey)
}

func NewOpenAIClient(apiKey string) (*OpenAIClient, error) {
	client := openai.NewClient(option.WithAPIKey(apiKey))

	document, schemaBytes, err := loadDiagnosisSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to parse embedded schema for OpenAI: %w", err)
	}
	var schemaMap map[string]any
	if err := json.Unmarshal(schemaBytes, &schemaMap); err != nil {
		return nil, fmt.Errorf("decode OpenAI schema: %w", err)
	}

	return &OpenAIClient{
		client:     &client,
		schemaMap:  schemaMap,
		schemaName: document.Name,
		schemaDesc: document.Description,
		strict:     document.Strict,
	}, nil
}

func (c *OpenAIClient) Analyze(ctx context.Context, cfg InferenceConfig, systemPrompt string, logContext []LogInput) (DiagnosisResult, error) {
	if cfg.MaxTokens < 0 {
		return DiagnosisResult{}, fmt.Errorf("%w: OpenAI MaxTokens must be non-negative; 0 uses the model default", ErrBadRequest)
	}

	logsJSON, err := json.Marshal(logContext)
	if err != nil {
		return DiagnosisResult{}, fmt.Errorf("failed to marshal log context: %w", err)
	}

	params := openai.ChatCompletionNewParams{
		Model: openai.ChatModel(cfg.Model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(string(logsJSON)),
		},
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
				Type: "json_schema",
				JSONSchema: openai.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:        c.schemaName,
					Description: param.NewOpt(c.schemaDesc),
					Strict:      param.NewOpt(c.strict),
					Schema:      c.schemaMap,
				},
			},
		},
	}

	applyOpenAIOptions(&params, cfg)

	resp, err := c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return DiagnosisResult{}, fmt.Errorf("openai inference failed: %w", classifyProviderError(err))
	}

	if len(resp.Choices) == 0 {
		return DiagnosisResult{}, fmt.Errorf("%w: empty response received from OpenAI", ErrInvalidOutput)
	}
	if resp.Choices[0].FinishReason == "length" {
		return DiagnosisResult{}, fmt.Errorf("%w: OpenAI reached its output token limit (MaxTokens=%d; 0 uses the model default); increase MaxTokens or request a shorter response", ErrInvalidOutput, cfg.MaxTokens)
	}

	result, err := ParseDiagnosisResult([]byte(resp.Choices[0].Message.Content))
	if err != nil {
		return DiagnosisResult{}, fmt.Errorf("invalid OpenAI response: %w", err)
	}

	return result, nil
}

func (c *OpenAIClient) Ready(ctx context.Context) error {
	if _, err := c.client.Models.List(ctx); err != nil {
		return fmt.Errorf("OpenAI model-list readiness check failed: %w", classifyProviderError(err))
	}
	return nil
}

// applyOpenAIOptions sets the sampling, reasoning, and token limit options present in cfg. Options that
// are not set are left out so the model's defaults apply. OpenAI has no top-k option, and
// IncludeThoughts has no equivalent, so both are ignored.
func applyOpenAIOptions(params *openai.ChatCompletionNewParams, cfg InferenceConfig) {
	if cfg.Temperature != nil {
		params.Temperature = param.NewOpt(float64(*cfg.Temperature))
	}
	if cfg.TopP != nil {
		params.TopP = param.NewOpt(float64(*cfg.TopP))
	}
	if cfg.MaxTokens > 0 {
		params.MaxCompletionTokens = param.NewOpt(int64(cfg.MaxTokens))
	}
	if cfg.Thinking != ThinkingDisabled {
		params.ReasoningEffort = shared.ReasoningEffort(cfg.Thinking)
	}
}

func (c *OpenAIClient) Close() error {
	// The OpenAI Go SDK client does not require an explicit teardown
	return nil
}
