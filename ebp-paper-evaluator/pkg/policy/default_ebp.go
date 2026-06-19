package policy

// BuiltinEBP21IR returns the versioned machine-readable defaults used when a
// policy Markdown file contains no inline policy-json and has no compiled
// sidecar. The caller's Markdown remains the authoritative policy text.
func BuiltinEBP21IR() PolicyIR {
	return PolicyIR{
		SchemaVersion: "1",
		ClaimTypes: []PolicyEnum{
			{ID: "mathematical_structure", Label: "Mathematical structure", Description: "Formal object or relation"},
			{ID: "physical_ontology", Label: "Physical ontology", Description: "Statement about physical reality"},
			{ID: "dynamics", Label: "Dynamics", Description: "Evolution or law"},
			{ID: "observable", Label: "Observable", Description: "Measurable consequence"},
			{ID: "interpretation", Label: "Interpretation", Description: "Meaning assigned to formalism"},
		},
		FunctionClasses: []PolicyEnum{
			{ID: "fundamental_candidate", Label: "Fundamental candidate", Description: "Candidate substrate or dynamics"},
			{ID: "reconstruction", Label: "Reconstruction mechanism", Description: "Reconstructs one structure from another"},
			{ID: "effective_theory", Label: "Effective theory", Description: "Regime-limited description"},
		},
		MaturityStages: []PolicyEnum{
			{ID: "seed", Label: "Seed", Description: "Speculative idea"},
			{ID: "model", Label: "Model", Description: "Partially structured proposal"},
			{ID: "framework", Label: "Framework", Description: "Declared concepts and maps"},
			{ID: "candidate_theory", Label: "Candidate theory", Description: "Claims controlled recovery"},
		},
		DebtItems: []DebtItem{
			{ID: "needMap", Label: "Map", Description: "Domain, codomain and translation rule", Automated: true, Required: true, DefaultStatus: DebtRemaining},
			{ID: "needInvariant", Label: "Invariant", Description: "Surviving property", Automated: true, Required: true, DefaultStatus: DebtRemaining},
			{ID: "needToyCheck", Label: "Toy check", Description: "Finite check", Automated: true, Required: true, DefaultStatus: DebtRemaining},
			{ID: "needNullModel", Label: "Null model", Description: "Simpler baseline", Automated: true, Required: true, DefaultStatus: DebtRemaining},
			{ID: "needObstruction", Label: "Obstruction", Description: "Known blocker treatment", Automated: true, Required: true, DefaultStatus: DebtRemaining},
			{ID: "needFaithfulnessReview", Label: "Faithfulness review", Description: "Human review of formal-to-physical mapping", Automated: false, Required: true, DefaultStatus: DebtNotAssessed},
		},
		StatusRules: []StatusRule{},
		HardFailures: []HardFailure{
			{ID: "forbidden_language", Flag: "forbidden_language", RewardCap: .2, BlocksReadiness: true, Patterns: []string{`(?i)\b(proved|verified truth|promoted research artifact|solved|validated physics)\b`}},
			{ID: "invented_source_quote", Flag: "invented_source_quote", RewardCap: 0, BlocksReadiness: true, Patterns: []string{}},
			{ID: "missing_evidence_span", Flag: "missing_evidence_span", RewardCap: 0, BlocksReadiness: true, Patterns: []string{}},
		},
		RubricDimensions: []RubricDimension{
			{ID: "source_support", Description: "Support in cited source", Weight: .25},
			{ID: "claim_type_correctness", Description: "Classification quality", Weight: .1},
			{ID: "map_quality", Description: "Map quality", Weight: .1},
			{ID: "invariant_quality", Description: "Invariant quality", Weight: .1},
			{ID: "toy_check_quality", Description: "Toy check quality", Weight: .1},
			{ID: "null_model_quality", Description: "Null model quality", Weight: .1},
			{ID: "obstruction_handling", Description: "Obstruction handling", Weight: .1},
			{ID: "bridge_validity", Description: "Bridge validity", Weight: .1},
			{ID: "uncertainty_disclosure", Description: "Uncertainty disclosure", Weight: .05},
		},
		RequiredOutputs: []RequiredOutput{
			{ID: "rationale", Field: "rationale", Type: "string", Required: true},
			{ID: "dimensions", Field: "dimension_scores", Type: "object", Required: true},
			{ID: "debts", Field: "debt_decisions", Type: "object", Required: true},
		},
		RoleInstructions: RoleInstructions{
			MetadataA: "Extract source-backed candidate claims.", MetadataB: "Challenge unsupported, duplicated or overbroad claims.", MetadataE: "Consolidate only claims with exact evidence spans.",
			WorkerA: "Build a constructive candidate assessment against the policy.", WorkerB: "Build an adversarial candidate assessment against the policy.", EvaluatorE: "Judge the complete candidate using the declared rubric and debt schema.",
		},
		ReportLanguage: ReportLanguagePolicy{
			StatusLabel:        "CANDIDATE_REPORT_COMPLETE_HUMAN_REVIEW_REQUIRED",
			RequiredStatements: []string{"Automated EBP profile excluding faithfulness review.", "Faithfulness review was not performed.", "This is an automated candidate assessment, not full EBP promotion."},
			ForbiddenPatterns:  []string{`(?i)\b(proved|verified truth|promoted research artifact|solved|validated physics)\b`},
		},
	}
}
