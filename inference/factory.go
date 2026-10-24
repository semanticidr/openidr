package inference

import (
	"context"
	"fmt"
	"strings"
)

type MultiProviderFactory struct{}

func NewMultiProviderFactory() *MultiProviderFactory {
	return &MultiProviderFactory{}
}

func (f *MultiProviderFactory) GetClient(ctx context.Context, provider ProviderType) (LLMClient, error) {
	switch provider {
	case ProviderGemini:
		return NewGeminiClientFromEnv(ctx)
	case ProviderOpenAI:
		return NewOpenAIClientFromEnv()
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", provider)
	}
}

type EnabledProviderFactory struct {
	enabled map[ProviderType]struct{}
	factory LLMFactory
}

func NewEnabledProviderFactory(providers []ProviderType, factory LLMFactory) *EnabledProviderFactory {
	enabled := make(map[ProviderType]struct{}, len(providers))
	for _, provider := range providers {
		enabled[provider] = struct{}{}
	}
	return &EnabledProviderFactory{enabled: enabled, factory: factory}
}

func (f *EnabledProviderFactory) GetClient(ctx context.Context, provider ProviderType) (LLMClient, error) {
	if _, ok := f.enabled[provider]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrProviderDisabled, provider)
	}
	return f.factory.GetClient(ctx, provider)
}

func ParseEnabledProviders(raw string) ([]ProviderType, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("LLM_PROVIDERS must enable at least one provider")
	}
	seen := make(map[ProviderType]struct{})
	providers := make([]ProviderType, 0)
	for value := range strings.SplitSeq(raw, ",") {
		provider := ProviderType(strings.TrimSpace(strings.ToLower(value)))
		if provider == "" {
			return nil, fmt.Errorf("LLM_PROVIDERS contains an empty provider")
		}
		switch provider {
		case ProviderGemini, ProviderOpenAI:
		default:
			return nil, fmt.Errorf("unsupported provider %q in LLM_PROVIDERS", provider)
		}
		if _, exists := seen[provider]; exists {
			return nil, fmt.Errorf("provider %q appears more than once in LLM_PROVIDERS", provider)
		}
		seen[provider] = struct{}{}
		providers = append(providers, provider)
	}
	return providers, nil
}

func CheckProvidersReady(ctx context.Context, providers []ProviderType, factory LLMFactory) error {
	for _, provider := range providers {
		client, err := factory.GetClient(ctx, provider)
		if err != nil {
			return fmt.Errorf("initialize %s provider: %w", provider, err)
		}
		if client == nil {
			return fmt.Errorf("initialize %s provider: factory returned a nil client", provider)
		}
		readyErr := client.Ready(ctx)
		closeErr := client.Close()
		if readyErr != nil {
			if closeErr != nil {
				return fmt.Errorf("%s provider is not ready: %w (also failed to close readiness client: %v)", provider, readyErr, closeErr)
			}
			return fmt.Errorf("%s provider is not ready: %w", provider, readyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close %s provider readiness client: %w", provider, closeErr)
		}
	}
	return nil
}
