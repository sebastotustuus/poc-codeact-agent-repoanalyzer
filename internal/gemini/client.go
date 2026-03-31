// Package gemini provides a thin wrapper around the Google GenAI SDK.
package gemini

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

const defaultModel = "gemini-3.1-flash-lite-preview"

type Client struct {
	sdk   *genai.Client
	model string
}

type ClientOption func(*Client)

func WithModel(model string) ClientOption {
	return func(c *Client) { c.model = model }
}

func NewClient(ctx context.Context, apiKey string, opts ...ClientOption) (*Client, error) {
	sdk, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("gemini: init client: %w", err)
	}

	c := &Client{sdk: sdk, model: defaultModel}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

func (c *Client) GenerateContent(ctx context.Context, contents []*genai.Content, config *genai.GenerateContentConfig) (string, error) {
	resp, err := c.sdk.Models.GenerateContent(ctx, c.model, contents, config)
	if err != nil {
		return "", fmt.Errorf("gemini: generate content: %w", err)
	}
	text := resp.Text()
	if text == "" {
		return "", fmt.Errorf("gemini: empty response")
	}
	return text, nil
}
