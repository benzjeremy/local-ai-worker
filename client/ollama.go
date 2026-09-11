package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OllamaClient interfaces with a local Ollama server instance.
type OllamaClient struct {
	baseURL    string
	httpClient *http.Client
}

// GenerateRequest represents the payload for /api/generate.
type GenerateRequest struct {
	Model   string   `json:"model"`
	Prompt  string   `json:"prompt"`
	System  string   `json:"system,omitempty"`
	Stream  bool     `json:"stream"`
	Images  []string `json:"images,omitempty"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// GenerateResponse represents the non-streaming response from /api/generate.
type GenerateResponse struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
}

// NewOllamaClient creates a client pointing to the specified URL (defaults to http://127.0.0.1:11434).
func NewOllamaClient(baseURL string) *OllamaClient {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	return &OllamaClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Ping checks whether the Ollama server is reachable.
func (c *OllamaClient) Ping(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/", nil)
	if err != nil {
		return false
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Generate sends a prompt to Ollama and returns the generated text.
func (c *OllamaClient) Generate(ctx context.Context, req GenerateRequest) (string, error) {
	req.Stream = false
	payload, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to encode generate request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/generate", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed communicating with Ollama at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, string(body))
	}

	var genResp GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return "", fmt.Errorf("failed to parse ollama response: %w", err)
	}

	if genResp.Response == "" && !genResp.Done {
		return "", errors.New("empty response received from ollama")
	}

	return genResp.Response, nil
}
