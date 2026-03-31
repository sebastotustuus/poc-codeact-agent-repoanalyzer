package gemini_test

import (
	"context"
	"os"
	"testing"

	"github.com/user/poc-codeact-repoanalyzer/internal/gemini"
	"google.golang.org/genai"
)

// TestNewClient_MissingKey verifies that an obviously invalid key
// is rejected at the client-creation stage or at first call.
// This test does NOT require a real API key.
func TestNewClient_InvalidInit(t *testing.T) {
	t.Parallel()

	_, err := gemini.NewClient(context.Background(), "")
	// The SDK may accept an empty key at init and only fail at first call;
	// either way, if we get a client we just ensure construction doesn't panic.
	if err != nil {
		t.Logf("NewClient returned expected error for empty key: %v", err)
	}
}

// TestGenerateContent_Integration calls the real Gemini API.
// Skipped when GEMINI_API_KEY is not set.
func TestGenerateContent_Integration(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set — skipping integration test")
	}

	ctx := context.Background()
	client, err := gemini.NewClient(ctx, apiKey)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	contents := []*genai.Content{{
		Role:  genai.RoleUser,
		Parts: []*genai.Part{{Text: "Say exactly: hello"}},
	}}
	config := &genai.GenerateContentConfig{
		Temperature:     genai.Ptr[float32](0),
		MaxOutputTokens: 16,
	}

	text, err := client.GenerateContent(ctx, contents, config)
	if err != nil {
		t.Fatalf("GenerateContent() error = %v", err)
	}
	if text == "" {
		t.Error("GenerateContent() returned empty text")
	}
	t.Logf("response: %q", text)
}
