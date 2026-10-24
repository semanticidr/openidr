// eval/inference/rca-engine/main.go
// Demonstrates generating an RCA report with RCAEngine.DiagnoseWithContext.
// Run with: GEMINI_API_KEY="your_key" go run eval/inference/rca-engine/main.go
// or with Vertex AI env creds (GOOGLE_APPLICATION_CREDENTIALS pointing to a service account key file).
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

	// DiagnoseWithContext receives logs directly, so no LogRetriever is needed.
	engine := inference.NewRCAEngine(nil, inference.NewMultiProviderFactory())

	cfg := inference.InferenceConfig{
		Provider:    inference.ProviderGemini,
		Model:       "gemini-3.1-flash-lite",
		Temperature: new(float32(0.1)),
	}

	mockLogs := []inference.LogInput{
		{ID: "log-1", Timestamp: time.Date(2026, 10, 2, 10, 5, 0, 0, time.UTC), Service: "payment-gateway-service", Message: "Upstream rate limit exceeded for api.stripe.com"},
		{ID: "log-2", Timestamp: time.Date(2026, 10, 2, 10, 5, 10, 0, time.UTC), Service: "payment-gateway-service", Message: "unable to process checkout session (stripe 429)"},
	}

	fmt.Println("Generating RCA report via RCAEngine...")

	result, err := engine.DiagnoseWithContext(ctx, mockLogs, cfg)
	if err != nil {
		log.Fatalf("RCA diagnosis failed: %v", err)
	}

	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatalf("Encode diagnosis: %v", err)
	}
	fmt.Printf("\n--- RCA REPORT ---\n%s\n", string(out))
}
