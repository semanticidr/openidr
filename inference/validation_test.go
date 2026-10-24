package inference

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/openai/openai-go"
	"google.golang.org/genai"
)

// StaticFactory hands out one prebuilt client whatever the provider.
type StaticFactory struct{ Client LLMClient }

func (f StaticFactory) GetClient(context.Context, ProviderType) (LLMClient, error) {
	if f.Client == nil {
		return nil, fmt.Errorf("no LLM client configured")
	}
	return f.Client, nil
}

func validResult() DiagnosisResult {
	return DiagnosisResult{
		OccurredAt: "2026-10-04T12:00:00Z", Title: "t", Summary: "s", RootCause: "r", Severity: 1,
		Timeline:           []Timeline{{Event: "e", LogIDS: []string{"a"}}},
		RecommendedActions: []RecommendedAction{{Type: Mitigation, Action: "do"}},
	}
}

func TestValidate(t *testing.T) {
	if err := (&DiagnosisResult{}).Validate(); err == nil {
		t.Fatal("empty result accepted")
	}
	ok := validResult()
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid result rejected: %v", err)
	}

	for name, change := range map[string]func(*DiagnosisResult){
		"no title":          func(r *DiagnosisResult) { r.Title = "" },
		"no summary":        func(r *DiagnosisResult) { r.Summary = "" },
		"no root cause":     func(r *DiagnosisResult) { r.RootCause = "" },
		"severity too high": func(r *DiagnosisResult) { r.Severity = 5 },
		"negative severity": func(r *DiagnosisResult) { r.Severity = -1 },
		"bad timestamp":     func(r *DiagnosisResult) { r.OccurredAt = "yesterday" },
		"unknown action":    func(r *DiagnosisResult) { r.RecommendedActions[0].Type = "panic" },
	} {
		r := validResult()
		change(&r)
		if err := r.Validate(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestParseDiagnosisResultErrorsAreInvalidOutput(t *testing.T) {
	for name, in := range map[string]string{"malformed": "{", "invalid": `{"title":""}`} {
		if _, err := ParseDiagnosisResult([]byte(in)); !errors.Is(err, ErrInvalidOutput) {
			t.Errorf("%s: got %v, want ErrInvalidOutput", name, err)
		}
	}
}

func TestDropUnknownLogs(t *testing.T) {
	r := DiagnosisResult{
		Timeline: []Timeline{{Event: "e1", LogIDS: []string{"a", "ghost", "b"}}, {Event: "e2", LogIDS: []string{"ghost"}}},
	}
	r.DropUnknownLogs([]LogInput{{ID: "a"}, {ID: "b"}})

	if !slices.Equal(r.Timeline[0].LogIDS, []string{"a", "b"}) || len(r.Timeline[1].LogIDS) != 0 {
		t.Errorf("timeline = %+v", r.Timeline)
	}
	if len(r.Timeline) != 2 {
		t.Error("events must be kept even when none of their logs are known")
	}
}

type reportingClient struct{ result DiagnosisResult }

func (c reportingClient) Analyze(context.Context, InferenceConfig, string, []LogInput) (DiagnosisResult, error) {
	return c.result, nil
}
func (reportingClient) Ready(context.Context) error { return nil }
func (reportingClient) Close() error                { return nil }

func TestEngineDropsInventedLogIDs(t *testing.T) {
	r := validResult()
	r.Timeline[0].LogIDS = []string{"a", "ghost"}
	engine := NewRCAEngine(nil, StaticFactory{Client: reportingClient{result: r}})

	got, err := engine.DiagnoseWithContext(t.Context(), []LogInput{validLogInput("a")}, InferenceConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Timeline[0].LogIDS, []string{"a"}) {
		t.Errorf("timeline ids = %v", got.Timeline[0].LogIDS)
	}
}

func TestClassifyProviderError(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want error
	}{
		"gemini 401": {genai.APIError{Code: http.StatusUnauthorized, Message: "bad key sk-secret"}, ErrAuth},
		"gemini 403": {genai.APIError{Code: http.StatusForbidden}, ErrAuth},
		"gemini 429": {genai.APIError{Code: http.StatusTooManyRequests, Message: "quota"}, ErrRateLimited},
		"gemini 400": {genai.APIError{Code: http.StatusBadRequest, Message: "unknown model"}, ErrBadRequest},
		"gemini 404": {genai.APIError{Code: http.StatusNotFound, Message: "model not found"}, ErrBadRequest},
		"gemini 503": {genai.APIError{Code: http.StatusServiceUnavailable}, ErrProviderUnavailable},
		"openai 401": {&openai.Error{StatusCode: http.StatusUnauthorized, Message: "Incorrect API key provided: sk-abc"}, ErrAuth},
		"openai 429": {&openai.Error{StatusCode: http.StatusTooManyRequests}, ErrRateLimited},
		"openai 400": {&openai.Error{StatusCode: http.StatusBadRequest, Message: "bad option"}, ErrBadRequest},
		"openai 500": {&openai.Error{StatusCode: http.StatusInternalServerError}, ErrProviderUnavailable},
		"network":    {errors.New("dial tcp: connection refused"), ErrProviderUnavailable},
		"wrapped":    {fmt.Errorf("outer: %w", genai.APIError{Code: http.StatusTooManyRequests}), ErrRateLimited},
	} {
		got := classifyProviderError(tc.err)
		if !errors.Is(got, tc.want) {
			t.Errorf("%s: got %v, want %v", name, got, tc.want)
		}
	}

	if got := classifyProviderError(genai.APIError{Code: http.StatusUnauthorized, Message: "bad key sk-secret"}); strings.Contains(got.Error(), "sk-secret") {
		t.Errorf("authentication errors must not carry provider detail: %v", got)
	}
	if got := classifyProviderError(genai.APIError{Code: http.StatusBadRequest, Message: strings.Repeat("x", 5000)}); len(got.Error()) > 500 {
		t.Errorf("provider detail is not truncated: %d bytes", len(got.Error()))
	}
	for _, ctxErr := range []error{context.Canceled, context.DeadlineExceeded} {
		if got := classifyProviderError(ctxErr); !errors.Is(got, ctxErr) || errors.Is(got, ErrProviderUnavailable) {
			t.Errorf("context error %v must pass through, got %v", ctxErr, got)
		}
	}
	if classifyProviderError(nil) != nil {
		t.Error("nil must stay nil")
	}
}

func TestGeminiGenerateConfigOnlySetsProvidedOptions(t *testing.T) {
	c := &GeminiClient{}

	unset := c.generateConfig(InferenceConfig{}, "prompt")
	if unset.Temperature != nil || unset.TopP != nil || unset.TopK != nil || unset.ThinkingConfig != nil {
		t.Errorf("unset options must be left out: %+v", unset)
	}

	cfg := c.generateConfig(InferenceConfig{
		Temperature: new(float32(0)), TopP: new(float32(0.9)), TopK: new(int32(40)),
		Thinking: ThinkingHigh, IncludeThoughts: true,
	}, "prompt")
	if cfg.Temperature == nil || *cfg.Temperature != 0 {
		t.Error("an explicit zero temperature must be sent")
	}
	if *cfg.TopP != 0.9 || *cfg.TopK != 40 {
		t.Errorf("top_p/top_k = %v/%v", *cfg.TopP, *cfg.TopK)
	}
	if cfg.ThinkingConfig == nil || cfg.ThinkingConfig.ThinkingLevel != genai.ThinkingLevelHigh || !cfg.ThinkingConfig.IncludeThoughts {
		t.Errorf("thinking = %+v", cfg.ThinkingConfig)
	}
	if cfg.SystemInstruction.Parts[0].Text != "prompt" || cfg.ResponseMIMEType != "application/json" {
		t.Error("prompt and response type must always be set")
	}

	// IncludeThoughts alone does nothing without a thinking level.
	if c.generateConfig(InferenceConfig{IncludeThoughts: true}, "p").ThinkingConfig != nil {
		t.Error("thinking must stay off when no level is set")
	}
}

func TestApplyOpenAIOptions(t *testing.T) {
	var unset openai.ChatCompletionNewParams
	applyOpenAIOptions(&unset, InferenceConfig{TopK: new(int32(40)), IncludeThoughts: true})
	if unset.Temperature.Valid() || unset.TopP.Valid() || unset.ReasoningEffort != "" {
		t.Errorf("unset options must be left out: %+v", unset)
	}

	var set openai.ChatCompletionNewParams
	applyOpenAIOptions(&set, InferenceConfig{Temperature: new(float32(0)), TopP: new(float32(0.5)), Thinking: ThinkingMedium})
	if !set.Temperature.Valid() || set.Temperature.Value != 0 {
		t.Error("an explicit zero temperature must be sent")
	}
	if set.TopP.Value != 0.5 || string(set.ReasoningEffort) != "medium" {
		t.Errorf("options = %+v", set)
	}
}

func TestProviderClientsReadTheirOwnEnvironment(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-openai-key")
	t.Setenv("GEMINI_AUTH_MODE", "api_key")
	t.Setenv("GEMINI_API_KEY", "test-gemini-key")

	factory := NewMultiProviderFactory()
	for _, provider := range []ProviderType{ProviderOpenAI, ProviderGemini} {
		client, err := factory.GetClient(context.Background(), provider)
		if err != nil {
			t.Fatalf("%s: %v", provider, err)
		}
		if err := client.Close(); err != nil {
			t.Fatalf("close %s: %v", provider, err)
		}
	}
}

func TestStaticFactory(t *testing.T) {
	if _, err := (StaticFactory{}).GetClient(context.Background(), ProviderOpenAI); err == nil {
		t.Error("a factory without a client must fail")
	}
	client := &MockLLMClient{}
	got, err := (StaticFactory{Client: client}).GetClient(context.Background(), ProviderGemini)
	if err != nil || got != LLMClient(client) {
		t.Errorf("got %v, %v", got, err)
	}
}

func validLogInput(id string) LogInput {
	return LogInput{ID: id, Timestamp: time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC), Service: "api", Message: "m"}
}

func TestEngineRejectsInvalidResults(t *testing.T) {
	bad := validResult()
	bad.Title = ""
	engine := NewRCAEngine(nil, StaticFactory{Client: reportingClient{result: bad}})
	_, err := engine.DiagnoseWithContext(t.Context(), []LogInput{validLogInput("a")}, InferenceConfig{})
	if !errors.Is(err, ErrInvalidOutput) {
		t.Fatalf("got %v, want ErrInvalidOutput", err)
	}
}
