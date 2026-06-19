package simple

import (
	"ebp-paper-evaluator/pkg/budget"
	"ebp-paper-evaluator/pkg/document"
	"ebp-paper-evaluator/pkg/llm"
	"ebp-paper-evaluator/pkg/policy"
	"time"
)

const (
	UserInstruction = "Apply EBP 2.1 on the attached physics paper."
	FinalStatus     = "CANDIDATE_EBP_ASSESSMENT_HUMAN_REVIEW_REQUIRED"
)

var RequiredLimitations = []string{
	"This is an automated candidate EBP assessment.",
	"It is not a proof of the paper's claims.",
	"It is not full EBP promotion.",
	"Human faithfulness review was not performed.",
	"The three LLM reviews are comparison signals, not authorities.",
}

type EvidenceQuote struct {
	Quote       string `json:"quote"`
	SectionHint string `json:"section_hint"`
}

type ReviewerClaim struct {
	ClaimID        string          `json:"claim_id"`
	ClaimText      string          `json:"claim_text"`
	EvidenceQuotes []EvidenceQuote `json:"evidence_quotes"`
	EBPDebts       []string        `json:"ebp_debts"`
	Status         string          `json:"status"`
}

type ReviewerOutput struct {
	ReviewerID             string          `json:"reviewer_id"`
	ModelID                string          `json:"model_id"`
	PaperSummary           string          `json:"paper_summary"`
	MainClaims             []ReviewerClaim `json:"main_claims"`
	MapsIdentified         []string        `json:"maps_identified"`
	InvariantsIdentified   []string        `json:"invariants_identified"`
	ToyChecksIdentified    []string        `json:"toy_checks_identified"`
	NullModelsIdentified   []string        `json:"null_models_identified"`
	ObstructionsIdentified []string        `json:"obstructions_identified"`
	FaithfulnessLimits     []string        `json:"faithfulness_limits"`
	OverclaimWarnings      []string        `json:"overclaim_warnings"`
	RecommendedNextSteps   []string        `json:"recommended_next_steps"`
	OverallAssessment      string          `json:"overall_assessment"`
	Limitations            []string        `json:"limitations"`
}

type Scores struct {
	ClaimCoverage               float64  `json:"claim_coverage"`
	ClaimCoverageMeaning        string   `json:"claim_coverage_meaning"`
	SourceGrounding             float64  `json:"source_grounding"`
	SourceGroundingNotes        []string `json:"source_grounding_notes,omitempty"`
	EBPDebtCoverage             float64  `json:"ebp_debt_coverage"`
	MapInvariantQuality         float64  `json:"map_invariant_quality"`
	ToyNullObstructionAwareness float64  `json:"toy_null_obstruction_awareness"`
	FaithfulnessHumility        float64  `json:"faithfulness_humility"`
	NoOverclaimDiscipline       float64  `json:"no_overclaim_discipline"`
	CrossModelAgreement         float64  `json:"cross_model_agreement"`
	NextStepUsefulness          float64  `json:"next_step_usefulness"`
}

type ReviewerScore struct {
	ReviewerID     string  `json:"reviewer_id"`
	CallStatus     string  `json:"call_status"`
	ParseStatus    string  `json:"parse_status"`
	Scores         Scores  `json:"scores"`
	ReviewerScore  float64 `json:"reviewer_score"`
	FinalScore     float64 `json:"final_score"`
	Status         string  `json:"status"`
	FailureAffects string  `json:"failure_affects,omitempty"`
}

type ReviewerError struct {
	ReviewerID   string `json:"reviewer_id"`
	ModelID      string `json:"model_id"`
	CallStatus   string `json:"call_status"`
	ErrorSummary string `json:"error_summary"`
	Retryable    bool   `json:"retryable"`
}

type ReviewerResult struct {
	ReviewerID string          `json:"reviewer_id"`
	ModelID    string          `json:"model_id"`
	Raw        string          `json:"-"`
	Parsed     *ReviewerOutput `json:"parsed,omitempty"`
	Score      ReviewerScore   `json:"score"`
	Error      *ReviewerError  `json:"error,omitempty"`
}

type ClaimCluster struct {
	CanonicalText string              `json:"canonical_text"`
	Reviewers     []string            `json:"reviewers"`
	Claims        map[string][]string `json:"claims"`
}

type AgreementLedger struct {
	SharedClaims           []ClaimCluster      `json:"shared_claims"`
	PartiallySharedClaims  []ClaimCluster      `json:"partially_shared_claims"`
	UniqueClaimsByReviewer map[string][]string `json:"unique_claims_by_reviewer"`
	ConflictingClaims      []string            `json:"conflicting_claims"`
	SharedDebts            []string            `json:"shared_debts"`
	UniqueDebtsByReviewer  map[string][]string `json:"unique_debts_by_reviewer"`
	SharedObstructions     []string            `json:"shared_obstructions"`
	AgreementScore         float64             `json:"agreement_score"`
	AgreementStatus        string              `json:"agreement_status"`
	PossibleHallucinations map[string][]string `json:"possible_hallucinations"`
}

type ScoringSummary struct {
	RunStatus                      string  `json:"run_status"`
	RunCompleteness                float64 `json:"run_completeness"`
	MeanReviewerScoreParseableOnly float64 `json:"mean_reviewer_score_parseable_only"`
	MeanReviewerScoreWithFailures  float64 `json:"mean_reviewer_score_with_failures"`
	ParseableReviewers             int     `json:"parseable_reviewers"`
	ReturnedReviewers              int     `json:"returned_reviewers"`
}

type Provenance struct {
	Timestamp        string            `json:"timestamp"`
	Mode             string            `json:"mode"`
	TreeQuestUsed    bool              `json:"treequest_used"`
	DocumentHash     string            `json:"document_hash"`
	PolicySourceHash string            `json:"policy_source_hash"`
	PolicyIRHash     string            `json:"policy_ir_hash"`
	PolicyIRSource   string            `json:"policy_ir_source"`
	ProfileID        string            `json:"profile_id"`
	Models           map[string]string `json:"models"`
	RunStatus        string            `json:"run_status"`
	RunCompleteness  float64           `json:"run_completeness"`
}

type Config struct {
	Document  document.DocumentBundle
	Policy    policy.PolicyBundle
	Profile   policy.EvaluationProfile
	Client    llm.LLMClient
	Tracker   *budget.Tracker
	Models    []string
	Out       string
	MaxTokens int
	Clock     func() time.Time
}

type Result struct {
	Reviewers  []ReviewerResult
	Agreement  AgreementLedger
	Summary    ScoringSummary
	Provenance Provenance
}
