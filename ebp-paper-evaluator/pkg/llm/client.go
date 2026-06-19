package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"

	openrouter "github.com/revrost/go-openrouter"
)

// Message is a role-annotated chat message
type GenerateRequest struct {
	Role           string            `json:"role"`
	Task           string            `json:"task"`
	System         string            `json:"system"`
	User           string            `json:"user"`
	Model          string            `json:"model"`
	Temperature    float64           `json:"temperature"`
	MaxTokens      int               `json:"max_tokens"`
	ResponseFormat string            `json:"response_format,omitempty"` // "", "json_object"
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// TokenUsage tracks token metrics returned by the LLM client
type TokenUsage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	CostUSD          float64 `json:"cost_usd"`
}

// GenerateResponse holds the completion output, metadata, and token stats
type GenerateResponse struct {
	Content      string     `json:"content"`
	ModelID      string     `json:"model_id"`
	Usage        TokenUsage `json:"usage"`
	FinishReason string     `json:"finish_reason"`
	PromptHash   string     `json:"prompt_hash"`
}

// LLMClient abstracts any LLM endpoint provider
type LLMClient interface {
	Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error)
}

// OpenRouterClient implements LLMClient using go-openrouter SDK
type OpenRouterClient struct {
	client *openrouter.Client
}

// NewOpenRouterClient creates a new OpenRouter client using the API key in environment
func NewOpenRouterClient(apiKeyEnv string) (*OpenRouterClient, error) {
	apiKey := os.Getenv(apiKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("openrouter API key env variable %q is empty", apiKeyEnv)
	}
	return &OpenRouterClient{
		client: openrouter.NewClient(apiKey),
	}, nil
}

// Generate executes a chat completion request on OpenRouter
func (c *OpenRouterClient) Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error) {
	messages := []openrouter.ChatCompletionMessage{{Role: "system", Content: openrouter.Content{Text: req.System}}, {Role: "user", Content: openrouter.Content{Text: req.User}}}
	promptHash := computePromptHash(req)

	// Create request
	openrouterReq := openrouter.ChatCompletionRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: float32(req.Temperature),
		MaxTokens:   req.MaxTokens,
	}
	if req.ResponseFormat == "json_object" {
		openrouterReq.ResponseFormat = &openrouter.ChatCompletionResponseFormat{
			Type: openrouter.ChatCompletionResponseFormatTypeJSONObject,
		}
	}

	resp, err := c.client.CreateChatCompletion(ctx, openrouterReq)
	if err != nil {
		return GenerateResponse{}, fmt.Errorf("openrouter request failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return GenerateResponse{}, fmt.Errorf("openrouter returned empty completion choices")
	}

	content := resp.Choices[0].Message.Content.Text
	finishReason := string(resp.Choices[0].FinishReason)

	usage := TokenUsage{}
	if resp.Usage != nil {
		usage.PromptTokens = resp.Usage.PromptTokens
		usage.CompletionTokens = resp.Usage.CompletionTokens
		usage.TotalTokens = resp.Usage.TotalTokens
	}

	return GenerateResponse{
		Content:      content,
		ModelID:      resp.Model,
		Usage:        usage,
		FinishReason: finishReason,
		PromptHash:   promptHash,
	}, nil
}

// MockLLMClient implements LLMClient with programmable canned responses
type MockLLMClient struct {
	CannedResponse string
	CustomMockFn   func(req GenerateRequest) (GenerateResponse, error)
}

func (m *MockLLMClient) Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error) {
	if m.CustomMockFn != nil {
		return m.CustomMockFn(req)
	}
	return GenerateResponse{
		Content: m.CannedResponse,
		ModelID: req.Model,
		Usage: TokenUsage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
			CostUSD:          0.0001,
		},
		FinishReason: "stop",
		PromptHash:   computePromptHash(req),
	}, nil
}

// Helper to compute prompt hash
func computePromptHash(req GenerateRequest) string {
	hasher := sha256.New()
	hasher.Write([]byte(req.Role + "\n" + req.Task + "\n" + req.System + "\n" + req.User + "\n" + req.Model + "\n" + req.ResponseFormat + "\n"))
	keys := make([]string, 0, len(req.Metadata))
	for k := range req.Metadata {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		hasher.Write([]byte(k + "=" + req.Metadata[k] + "\n"))
	}
	return hex.EncodeToString(hasher.Sum(nil))
}
