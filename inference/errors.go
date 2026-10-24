package inference

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/openai/openai-go"
	"google.golang.org/genai"
)

// Errors returned by the LLM clients and the engine, so callers can react to the kind of failure.
// Use errors.Is; the wrapped error carries the provider's own detail.
var (
	// ErrAuth means the provider rejected the credentials.
	ErrAuth = errors.New("model provider rejected the credentials")
	// ErrRateLimited means the provider throttled the request or the quota is exhausted.
	ErrRateLimited = errors.New("model provider rate limit exceeded")
	// ErrBadRequest means the provider rejected the request itself, for example an unknown model or an
	// option the model does not support.
	ErrBadRequest = errors.New("model provider rejected the request")
	// ErrProviderUnavailable means the provider could not be reached or failed on its side.
	ErrProviderUnavailable = errors.New("model provider unavailable")
	// ErrProviderDisabled means the deployment does not enable the selected provider.
	ErrProviderDisabled = errors.New("model provider is not enabled")
	// ErrInvalidOutput means the model answered, but not with a usable diagnosis.
	ErrInvalidOutput = errors.New("model returned an invalid result")
)

const maxProviderDetail = 300

// classifyProviderError wraps an error from a provider SDK in one of the sentinel errors above.
// Context cancellation and deadlines are returned unchanged.
func classifyProviderError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	status, detail := 0, ""
	var gemini genai.APIError
	var openaiErr *openai.Error
	switch {
	case errors.As(err, &gemini):
		status, detail = gemini.Code, gemini.Message
	case errors.As(err, &openaiErr):
		status, detail = openaiErr.StatusCode, openaiErr.Message
	default:
		// Network failures and anything the SDK did not turn into an HTTP status.
		return fmt.Errorf("%w: %w", ErrProviderUnavailable, err)
	}

	var kind error
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		kind = ErrAuth
		// Providers can echo part of the key in authentication errors, so no detail is kept.
		return kind
	case status == http.StatusTooManyRequests:
		kind = ErrRateLimited
	case status >= 400 && status < 500:
		kind = ErrBadRequest
	default:
		kind = ErrProviderUnavailable
	}
	return fmt.Errorf("%w: %s", kind, truncate(strings.TrimSpace(detail), maxProviderDetail))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
