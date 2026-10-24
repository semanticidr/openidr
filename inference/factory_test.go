package inference

import (
	"context"
	"errors"
	"testing"
)

func TestMultiProviderFactoryReadsProviderCredentialsFromEnv(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GEMINI_AUTH_MODE", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	t.Setenv("GOOGLE_CLOUD_LOCATION", "")

	factory := NewMultiProviderFactory()
	if _, err := factory.GetClient(context.Background(), ProviderGemini); err == nil {
		t.Fatal("Gemini client creation must fail when GEMINI_AUTH_MODE is missing")
	}
	if _, err := factory.GetClient(context.Background(), ProviderOpenAI); err == nil {
		t.Fatal("OpenAI client creation must fail when OPENAI_API_KEY is missing")
	}

	t.Setenv("OPENAI_API_KEY", "test-openai-key")
	t.Setenv("GEMINI_AUTH_MODE", "api_key")
	t.Setenv("GEMINI_API_KEY", "test-gemini-key")
	for _, provider := range []ProviderType{ProviderOpenAI, ProviderGemini} {
		client, err := factory.GetClient(context.Background(), provider)
		if err != nil {
			t.Fatalf("%s client creation failed: %v", provider, err)
		}
		if err := client.Close(); err != nil {
			t.Fatalf("closing %s client failed: %v", provider, err)
		}
	}
}

func TestParseEnabledProviders(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  []ProviderType
		fail  bool
	}{
		{"openai,gemini", []ProviderType{ProviderOpenAI, ProviderGemini}, false},
		{" GEMINI ", []ProviderType{ProviderGemini}, false},
		{"", nil, true},
		{"openai,", nil, true},
		{"openai,openai", nil, true},
		{"anthropic", nil, true},
	} {
		got, err := ParseEnabledProviders(tc.input)
		if tc.fail {
			if err == nil {
				t.Errorf("ParseEnabledProviders(%q) succeeded; want an error", tc.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseEnabledProviders(%q): %v", tc.input, err)
			continue
		}
		if len(got) != len(tc.want) {
			t.Errorf("ParseEnabledProviders(%q) = %v, want %v", tc.input, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("ParseEnabledProviders(%q) = %v, want %v", tc.input, got, tc.want)
				break
			}
		}
	}
}

type readinessTestClient struct {
	readyErr error
	closeErr error
	closed   bool
}

func (c *readinessTestClient) Ready(context.Context) error { return c.readyErr }
func (c *readinessTestClient) Analyze(context.Context, InferenceConfig, string, []LogInput) (DiagnosisResult, error) {
	return DiagnosisResult{}, nil
}
func (c *readinessTestClient) Close() error {
	c.closed = true
	return c.closeErr
}

type readinessTestFactory struct {
	client LLMClient
	err    error
}

func (f readinessTestFactory) GetClient(context.Context, ProviderType) (LLMClient, error) {
	return f.client, f.err
}

func TestCheckProvidersReady(t *testing.T) {
	providerErr := errors.New("not authenticated")
	client := &readinessTestClient{readyErr: providerErr}
	err := CheckProvidersReady(context.Background(), []ProviderType{ProviderOpenAI}, readinessTestFactory{client: client})
	if !errors.Is(err, providerErr) {
		t.Fatalf("got %v, want wrapped readiness error", err)
	}

	if !client.closed {
		t.Fatal("readiness client must be closed after a failed check")
	}

	if err := CheckProvidersReady(context.Background(), nil, readinessTestFactory{}); err != nil {
		t.Fatalf("disabled providers should not be checked: %v", err)
	}
}

func TestEnabledProviderFactoryRejectsDisabledProvider(t *testing.T) {
	factory := NewEnabledProviderFactory(
		[]ProviderType{ProviderOpenAI},
		NewMultiProviderFactory(),
	)
	if _, err := factory.GetClient(context.Background(), ProviderGemini); err == nil {
		t.Fatal("disabled providers must not create clients")
	}
}
