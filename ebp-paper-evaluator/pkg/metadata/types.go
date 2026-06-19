package metadata

import (
	"context"
	"ebp-paper-evaluator/pkg/document"
	"ebp-paper-evaluator/pkg/llm"
)

type PaperProfile struct {
	DocumentID string         `json:"document_id"`
	PolicyID   string         `json:"policy_id"`
	Claims     []ClaimRecord  `json:"claims"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}
type ClaimRecord struct {
	ID            string                  `json:"id"`
	Text          string                  `json:"text"`
	ClaimType     string                  `json:"claim_type"`
	FunctionClass string                  `json:"function_class"`
	MaturityStage string                  `json:"maturity_stage"`
	EvidenceSpans []document.EvidenceSpan `json:"evidence_spans"`
	Confidence    float64                 `json:"confidence"`
	SourceHash    string                  `json:"source_hash"`
}
type RoleClients struct{ WorkerA, WorkerB, Evaluator llmClient }
type llmClient interface {
	Generate(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error)
}
