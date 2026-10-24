// cmd/eval_gemini/main.go
// Run with: GEMINI_API_KEY="your_key" go run eval/inference/gemini_simple/main.go
// or with vertex ai env creds
// Set the GOOGLE_APPLICATION_CREDENTIALS environment variable to point to your Vertex AI service account key file.
// go run eval/inference/gemini_simple/main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/semanticidr/openidr/inference"
)

func main() {
	ctx := context.Background()

	// 1. Initialize the MultiProviderFactory
	factory := inference.NewMultiProviderFactory()

	// 2. Fetch the Gemini client
	client, err := factory.GetClient(ctx, inference.ProviderGemini)
	if err != nil {
		log.Fatalf("Failed to initialize Gemini client: %v", err)
	}
	defer client.Close()

	// 3. Configure inference parameters
	cfg := inference.InferenceConfig{
		Provider:    inference.ProviderGemini,
		Model:       "gemini-3.1-flash-lite",
		Temperature: new(float32(0.1)),
	}

	// 4. Set up the test payload
	systemPrompt := `You are an expert site reliability engineer.
Analyze the provided JSON logs and determine the root cause of the incident.
Format your response exactly matching the provided JSON schema.`

	mockLogs := []inference.LogInput{
		{ID: "log-1", Timestamp: time.Date(2026, 10, 2, 10, 5, 0, 0, time.UTC), Service: "payment-gateway-service", Message: "Upstream rate limit exceeded for api.stripe.com"},
		{ID: "log-2", Timestamp: time.Date(2026, 10, 2, 10, 5, 10, 0, time.UTC), Service: "payment-gateway-service", Message: "unable to process checkout session (stripe 429)"},
	}

	fmt.Println("Sending request to Google Gemini API...")

	// 5. Execute
	result, err := client.Analyze(ctx, cfg, systemPrompt, mockLogs)
	if err != nil {
		log.Fatalf("Gemini inference failed: %v", err)
	}

	// 6. Print the strongly typed struct as indented JSON
	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatalf("Encode diagnosis: %v", err)
	}
	fmt.Printf("\n--- GEMINI STRUCTURED OUTPUT ---\n%s\n", string(out))
}
