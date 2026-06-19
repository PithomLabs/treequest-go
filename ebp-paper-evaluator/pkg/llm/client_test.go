package llm

import (
	"context"
	"testing"
)

func TestComputePromptHash(t *testing.T) {
	r := GenerateRequest{Role: "worker_a", Task: "worker_constructive", System: "s", User: "u", Model: "m", Metadata: map[string]string{"b": "2", "a": "1"}}
	a := computePromptHash(r)
	r.Metadata = map[string]string{"a": "1", "b": "2"}
	if a != computePromptHash(r) {
		t.Fatal("metadata ordering changed hash")
	}
	r.Task = "other"
	if a == computePromptHash(r) {
		t.Fatal("task missing from hash")
	}
}
func TestMockClient_UsesRoleAndTask(t *testing.T) {
	m := MockLLMClient{CustomMockFn: func(r GenerateRequest) (GenerateResponse, error) {
		return GenerateResponse{Content: r.Role + "/" + r.Task}, nil
	}}
	x, e := m.Generate(context.Background(), GenerateRequest{Role: "metadata_a", Task: "metadata_extract", System: "arbitrary"})
	if e != nil || x.Content != "metadata_a/metadata_extract" {
		t.Fatal(x, e)
	}
}
func TestMockClient_DoesNotDependOnPromptSubstring(t *testing.T) {
	m := MockLLMClient{CustomMockFn: func(r GenerateRequest) (GenerateResponse, error) { return GenerateResponse{Content: r.Task}, nil }}
	a, _ := m.Generate(context.Background(), GenerateRequest{Task: "metadata_extract", System: "evaluator_judgment"})
	b, _ := m.Generate(context.Background(), GenerateRequest{Task: "metadata_extract", System: "completely changed"})
	if a.Content != b.Content {
		t.Fatal("prompt changed routing")
	}
}
