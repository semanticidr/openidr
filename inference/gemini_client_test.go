package inference

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"google.golang.org/genai"
)

func TestGeminiAnalyzeRejectsInvalidMaxTokens(t *testing.T) {
	limits := []int{-1}
	if strconv.IntSize == 64 {
		overflow := int64(math.MaxInt32) + 1
		limits = append(limits, int(overflow))
	}
	for _, limit := range limits {
		t.Run(strconv.Itoa(limit), func(t *testing.T) {
			// A nil SDK client ensures validation happens before any provider call.
			client := &GeminiClient{}
			_, err := client.Analyze(t.Context(), InferenceConfig{MaxTokens: limit}, "prompt", nil)
			if !errors.Is(err, ErrBadRequest) {
				t.Fatalf("got %v, want ErrBadRequest", err)
			}
			if !strings.Contains(err.Error(), "MaxTokens") || !strings.Contains(err.Error(), "0 uses the model default") {
				t.Fatalf("missing validation guidance: %v", err)
			}
		})
	}
}

func TestGeminiAnalyzeMaxTokens(t *testing.T) {
	validJSON, err := json.Marshal(validResult())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		limit     int
		finish    genai.FinishReason
		text      string
		truncated bool
	}{
		{"default", 0, genai.FinishReasonStop, string(validJSON), false},
		{"explicit", 1024, genai.FinishReasonStop, string(validJSON), false},
		{"int32 maximum", math.MaxInt32, genai.FinishReasonStop, string(validJSON), false},
		{"truncated JSON", 128, genai.FinishReasonMaxTokens, `{"title":`, true},
		{"empty output", 128, genai.FinishReasonMaxTokens, "", true},
		{"complete JSON at limit", 128, genai.FinishReasonMaxTokens, string(validJSON), true},
		{"default limit reached", 0, genai.FinishReasonMaxTokens, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				var request struct {
					GenerationConfig struct {
						MaxOutputTokens *int32 `json:"maxOutputTokens"`
					} `json:"generationConfig"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Errorf("decode request: %v", err)
					http.Error(w, "invalid request", http.StatusBadRequest)
					return
				}
				got := request.GenerationConfig.MaxOutputTokens
				if tc.limit == 0 {
					if got != nil {
						t.Errorf("default limit must be omitted, got %d", *got)
					}
				} else if got == nil || *got != int32(tc.limit) {
					t.Errorf("maxOutputTokens = %v, want %d", got, tc.limit)
				}
				w.Header().Set("Content-Type", "application/json")
				response := &genai.GenerateContentResponse{
					Candidates: []*genai.Candidate{{
						FinishReason: tc.finish,
						Content: &genai.Content{
							Role:  "model",
							Parts: []*genai.Part{{Text: tc.text}},
						},
					}},
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Errorf("encode response: %v", err)
				}
			}))
			defer server.Close()

			sdk, err := genai.NewClient(t.Context(), &genai.ClientConfig{
				Backend:     genai.BackendGeminiAPI,
				APIKey:      "test-key",
				HTTPClient:  server.Client(),
				HTTPOptions: genai.HTTPOptions{BaseURL: server.URL},
			})
			if err != nil {
				t.Fatal(err)
			}
			client, err := newGeminiClient(sdk)
			if err != nil {
				t.Fatal(err)
			}
			got, err := client.Analyze(t.Context(), InferenceConfig{
				Model: "gemini-test", MaxTokens: tc.limit,
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
