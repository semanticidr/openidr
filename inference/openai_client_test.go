package inference

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

func TestOpenAIAnalyzeRejectsInvalidMaxTokens(t *testing.T) {
	// A nil SDK client ensures validation happens before any provider call.
	client := &OpenAIClient{}
	_, err := client.Analyze(t.Context(), InferenceConfig{MaxTokens: -1}, "prompt", nil)
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("got %v, want ErrBadRequest", err)
	}
	if !strings.Contains(err.Error(), "MaxTokens") || !strings.Contains(err.Error(), "0 uses the model default") {
		t.Fatalf("missing validation guidance: %v", err)
	}
}

func TestOpenAIAnalyzeMaxTokens(t *testing.T) {
	validJSON, err := json.Marshal(validResult())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		limit     int
		finish    string
		text      string
		truncated bool
	}{
		{"default", 0, "stop", string(validJSON), false},
		{"explicit", 1024, "stop", string(validJSON), false},
		{"maximum int", int(^uint(0) >> 1), "stop", string(validJSON), false},
		{"truncated JSON", 128, "length", `{"title":`, true},
		{"empty output", 128, "length", "", true},
		{"complete JSON at limit", 128, "length", string(validJSON), true},
		{"default limit reached", 0, "length", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				var request struct {
					MaxCompletionTokens *int64 `json:"max_completion_tokens"`
					MaxTokens           *int64 `json:"max_tokens"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Errorf("decode request: %v", err)
					http.Error(w, "invalid request", http.StatusBadRequest)
					return
				}
				if request.MaxTokens != nil {
					t.Error("deprecated max_tokens must not be sent")
				}
				got := request.MaxCompletionTokens
				if tc.limit == 0 {
					if got != nil {
						t.Errorf("default limit must be omitted, got %d", *got)
					}
				} else if got == nil || *got != int64(tc.limit) {
					t.Errorf("max_completion_tokens = %v, want %d", got, tc.limit)
				}
				w.Header().Set("Content-Type", "application/json")
				response := openai.ChatCompletion{
					Choices: []openai.ChatCompletionChoice{{
						FinishReason: tc.finish,
						Message: openai.ChatCompletionMessage{
							Role: "assistant", Content: tc.text,
						},
					}},
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Errorf("encode response: %v", err)
				}
			}))
			defer server.Close()

			client, err := NewOpenAIClient("test-key")
			if err != nil {
				t.Fatal(err)
			}
			sdk := openai.NewClient(
				option.WithAPIKey("test-key"),
				option.WithBaseURL(server.URL+"/"),
				option.WithHTTPClient(server.Client()),
			)
			client.client = &sdk
			got, err := client.Analyze(t.Context(), InferenceConfig{
				Model: "openai-test", MaxTokens: tc.limit,
			}, "prompt", nil)
			if requests != 1 {
				t.Fatalf("got %d requests, want 1", requests)
			}
			if tc.truncated {
				if !errors.Is(err, ErrInvalidOutput) {
					t.Fatalf("got %v, want ErrInvalidOutput", err)
				}
				for _, message := range []string{"output token limit", "MaxTokens=" + strconv.Itoa(tc.limit), "increase MaxTokens", "shorter response"} {
					if !strings.Contains(err.Error(), message) {
						t.Errorf("error %q missing %q", err, message)
					}
				}
				if !reflect.DeepEqual(got, DiagnosisResult{}) {
					t.Errorf("truncated response returned a result: %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, validResult()) {
				t.Errorf("got %+v, want %+v", got, validResult())
			}
		})
	}
}
