package eval

import (
	"ebp-paper-evaluator/pkg/metadata"
	"ebp-paper-evaluator/pkg/policy"
	"encoding/json"
)

type RoleAnalysis struct {
	Role            string   `json:"role"`
	Analysis        string   `json:"analysis"`
	EvidenceSpanIDs []string `json:"evidence_span_ids"`
}
type DebtDecision struct {
	ItemID          string            `json:"item_id"`
	Status          policy.DebtStatus `json:"status"`
	Rationale       string            `json:"rationale"`
	EvidenceSpanIDs []string          `json:"evidence_span_ids"`
	Confidence      float64           `json:"confidence"`
}
type EvaluatorJudgment struct {
	DimensionScores map[string]float64      `json:"dimension_scores"`
	DebtDecisions   map[string]DebtDecision `json:"debt_decisions"`
	Flags           []string                `json:"flags"`
	Rationale       string                  `json:"rationale"`
}
type ValidationFlag struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}
type RewardCap struct {
	FailureID string  `json:"failure_id"`
	Cap       float64 `json:"cap"`
}
type ValidationResult struct {
	Passed           bool             `json:"passed"`
	Flags            []ValidationFlag `json:"flags"`
	RewardCaps       []RewardCap      `json:"reward_caps"`
	ReadinessBlocked bool             `json:"readiness_blocked"`
}
type RewardBreakdown struct {
	Dimensions    map[string]float64 `json:"dimensions"`
	WeightedBase  float64            `json:"weighted_base"`
	AppliedCaps   []RewardCap        `json:"applied_caps"`
	DerivedReward float64            `json:"derived_reward"`
}
type AssessmentState struct {
	DocumentID           string                  `json:"document_id"`
	DocumentHash         string                  `json:"document_hash"`
	PolicyID             string                  `json:"policy_id"`
	PolicySourceHash     string                  `json:"policy_source_hash"`
	PolicyIRHash         string                  `json:"policy_ir_hash"`
	ProfileID            string                  `json:"profile_id"`
	Claim                metadata.ClaimRecord    `json:"claim"`
	Debts                map[string]DebtDecision `json:"debts"`
	ConstructiveAnalysis RoleAnalysis            `json:"constructive_analysis"`
	AdversarialAnalysis  RoleAnalysis            `json:"adversarial_analysis"`
	EvaluatorJudgment    EvaluatorJudgment       `json:"evaluator_judgment"`
	Validation           ValidationResult        `json:"validation"`
	Reward               RewardBreakdown         `json:"reward"`
	AutomatedReviewReady bool                    `json:"automated_review_ready"`
	GenerationDepth      int                     `json:"generation_depth"`
	BudgetExhausted      bool                    `json:"budget_exhausted"`
}

func (s AssessmentState) Clone() AssessmentState {
	b, _ := json.Marshal(s)
	var x AssessmentState
	_ = json.Unmarshal(b, &x)
	return x
}
