package policy

type DebtStatus string

const (
	DebtRemaining     DebtStatus = "remaining"
	DebtRetired       DebtStatus = "retired"
	DebtNotApplicable DebtStatus = "not_applicable"
	DebtNotAssessed   DebtStatus = "not_assessed"
)

type PolicyEnum struct{ ID, Label, Description string }
type DebtItem struct {
	ID            string     `json:"id"`
	Label         string     `json:"label"`
	Description   string     `json:"description"`
	Automated     bool       `json:"automated"`
	Required      bool       `json:"required"`
	DefaultStatus DebtStatus `json:"default_status"`
}
type StatusRule struct {
	ID                   string                  `json:"id"`
	RequiredDebtStatuses map[string][]DebtStatus `json:"required_debt_statuses"`
	OutputStatus         string                  `json:"output_status"`
}
type HardFailure struct {
	ID              string   `json:"id"`
	Flag            string   `json:"flag"`
	RewardCap       float64  `json:"reward_cap"`
	BlocksReadiness bool     `json:"blocks_readiness"`
	Patterns        []string `json:"patterns,omitempty"`
}
type RubricDimension struct {
	ID          string  `json:"id"`
	Description string  `json:"description"`
	Weight      float64 `json:"weight"`
}
type RequiredOutput struct {
	ID       string `json:"id"`
	Field    string `json:"field"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}
type RoleInstructions struct {
	MetadataA  string `json:"metadata_a"`
	MetadataB  string `json:"metadata_b"`
	MetadataE  string `json:"metadata_e"`
	WorkerA    string `json:"worker_a"`
	WorkerB    string `json:"worker_b"`
	EvaluatorE string `json:"evaluator_e"`
}
type ReportLanguagePolicy struct {
	StatusLabel        string   `json:"status_label"`
	RequiredStatements []string `json:"required_statements"`
	ForbiddenPatterns  []string `json:"forbidden_patterns"`
}

type PolicyIR struct {
	SchemaVersion    string               `json:"schema_version"`
	ClaimTypes       []PolicyEnum         `json:"claim_types"`
	FunctionClasses  []PolicyEnum         `json:"function_classes"`
	MaturityStages   []PolicyEnum         `json:"maturity_stages"`
	DebtItems        []DebtItem           `json:"debt_items"`
	StatusRules      []StatusRule         `json:"status_rules"`
	HardFailures     []HardFailure        `json:"hard_failures"`
	RubricDimensions []RubricDimension    `json:"rubric_dimensions"`
	RequiredOutputs  []RequiredOutput     `json:"required_outputs"`
	RoleInstructions RoleInstructions     `json:"role_instructions"`
	ReportLanguage   ReportLanguagePolicy `json:"report_language"`
}

type PolicyBundle struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Version    string   `json:"version"`
	SourcePath string   `json:"source_path"`
	SourceHash string   `json:"source_hash"`
	IRHash     string   `json:"ir_hash"`
	Markdown   string   `json:"markdown,omitempty"`
	IR         PolicyIR `json:"ir"`
	Trusted    bool     `json:"trusted"`
}

type DebtOverride struct {
	DefaultStatus                DebtStatus `json:"default_status"`
	IncludedInAutomatedReadiness bool       `json:"included_in_automated_readiness"`
}
type EvaluationProfile struct {
	ID            string                  `json:"id"`
	Name          string                  `json:"name"`
	DebtOverrides map[string]DebtOverride `json:"debt_overrides"`
	StatusLabel   string                  `json:"status_label"`
	RequiredNotes []string                `json:"required_notes"`
}
