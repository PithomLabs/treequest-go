package budget

import (
	"context"
	"ebp-paper-evaluator/pkg/llm"
	"sync"
)

type PromptRecord struct {
	Role       string `json:"role"`
	Task       string `json:"task"`
	ModelID    string `json:"model_id"`
	PromptHash string `json:"prompt_hash"`
}
type Tracker struct {
	mu               sync.Mutex
	PromptTokens     int            `json:"prompt_tokens"`
	CompletionTokens int            `json:"completion_tokens"`
	TotalTokens      int            `json:"total_tokens"`
	CostUSD          float64        `json:"cost_usd"`
	Calls            map[string]int `json:"calls"`
	PromptRecords    []PromptRecord `json:"prompt_records"`
}

type UsageSnapshot struct {
	PromptTokens     int            `json:"prompt_tokens"`
	CompletionTokens int            `json:"completion_tokens"`
	TotalTokens      int            `json:"total_tokens"`
	CostUSD          float64        `json:"cost_usd"`
	Calls            map[string]int `json:"calls"`
	PromptRecords    []PromptRecord `json:"prompt_records"`
}

func New() *Tracker { return &Tracker{Calls: map[string]int{}} }
func (t *Tracker) Snapshot() UsageSnapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	return UsageSnapshot{PromptTokens: t.PromptTokens, CompletionTokens: t.CompletionTokens, TotalTokens: t.TotalTokens, CostUSD: t.CostUSD, Calls: clone(t.Calls), PromptRecords: append([]PromptRecord(nil), t.PromptRecords...)}
}
func clone(m map[string]int) map[string]int {
	o := map[string]int{}
	for k, v := range m {
		o[k] = v
	}
	return o
}

type Client struct {
	Inner   llm.LLMClient
	Tracker *Tracker
	Role    string
}

func (c Client) Generate(ctx context.Context, r llm.GenerateRequest) (llm.GenerateResponse, error) {
	x, e := c.Inner.Generate(ctx, r)
	if e != nil {
		return x, e
	}
	c.Tracker.mu.Lock()
	defer c.Tracker.mu.Unlock()
	c.Tracker.PromptTokens += x.Usage.PromptTokens
	c.Tracker.CompletionTokens += x.Usage.CompletionTokens
	c.Tracker.TotalTokens += x.Usage.TotalTokens
	c.Tracker.CostUSD += x.Usage.CostUSD
	c.Tracker.Calls[c.Role]++
	role := r.Role
	if role == "" {
		role = c.Role
	}
	c.Tracker.PromptRecords = append(c.Tracker.PromptRecords, PromptRecord{Role: role, Task: r.Task, ModelID: x.ModelID, PromptHash: x.PromptHash})
	return x, nil
}
