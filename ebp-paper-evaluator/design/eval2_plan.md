# Implementation Plan - EBP-EVAL-SIMPLE-0006: Real-Provider Degraded Smoke Acceptance and Agreement Diagnostics

## 1. Verdict and Scope

This plan addresses the ticket **EBP-EVAL-SIMPLE-0006: Real-Provider Degraded Smoke Acceptance and Agreement Diagnostics**.
All changes are contained strictly within the `ebp-paper-evaluator` codebase. There will be absolutely no modifications to the `treequest-go` repository.

## 2. Latest Smoke-Run Interpretation

The latest real-provider run (`/home/chaschel/Desktop/physics/sakana/out/my-paper-triple-review`) successfully demonstrated that the schema-contract hardening fixes the parsing issues:
* **Reviewers 2 and 3** (`poolside/laguna-m.1:free` and `nex-agi/nex-n2-pro:free`) successfully parsed and scored above `0.70`.
* **Reviewer 1** (`google/gemma-4-31b-it:free`) failed at the call level.
* **Lexical Agreement was 0.000:** Jaccard-based lexical claim matching yielded no overlap, though both models successfully extracted claims.

We accept this degraded run as operationally usable and will add clear diagnostics and status mapping to make this state honest, inspectable, and useful.

## 3. Files/Functions Likely Affected

* **[pkg/simple/types.go](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/simple/types.go):**
  * Update `Provenance` and `RunStatusSummary` structs to include prompt observability hashes, temperature, and `RealProviderStatus`.
  * Define `ModelSuitabilityLedger` and `ReviewerSuitability` structs.
  * Define `AgreementDiagnostics` and associated sub-structs (`ClaimPair`, `PairwiseDiagnostic`, `MatchingMethod`).
  * Add these ledger structs as fields to the main `Result` struct.
* **[pkg/simple/agreement.go](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/simple/agreement.go):**
  * Implement `BuildAgreementDiagnostics` to compute pairwise claim similarities and output diagnostics.
* **[pkg/simple/triple_review.go](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/simple/triple_review.go):**
  * Implement `determineRealProviderStatus` helper.
  * Implement `buildModelSuitability` helper.
  * Update `Run` to compile hashes (`user_message_hash`, `schema_example_hash`, `system_prompt_hash`) and populate `Provenance` and `Result` fields.
* **[pkg/simple/artifacts.go](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/simple/artifacts.go):**
  * Update `saveArtifacts` to write `consensus/agreement_diagnostics.json` and `consensus/model_suitability.json`.
  * Update `renderReport` to dynamically display `degraded_but_usable` status notes and Model Operational Notes.
* **[pkg/simple/triple_review_test.go](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/simple/triple_review_test.go):**
  * Add unit tests for `RealProviderStatus`, `ModelSuitabilityLedger`, and `AgreementDiagnostics`.

---

## 4. Real-Provider Status Design

The status `real_provider_status` is determined dynamically based on response metrics:

* **`full_triple_review_ready`:** 3 returned, 3 parsed.
* **`degraded_but_usable`:** $\ge 2$ returned, $\ge 2$ parsed.
* **`diagnostic_only`:** $> 0$ returned, 0 parsed.
* **`not_usable`:** 0 returned, or other fatal failure.

We will add a helper function `determineRealProviderStatus(returned, parseable int, assessmentStatus string) string` inside [triple_review.go](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/simple/triple_review.go).

---

## 5. Model Suitability Ledger Design

We will generate `consensus/model_suitability.json` to log model operational statistics. Structs:

```go
type ReviewerSuitability struct {
	ReviewerID  string  `json:"reviewer_id"`
	ModelID     string  `json:"model_id"`
	CallStatus  string  `json:"call_status"`
	ParseStatus string  `json:"parse_status"`
	Score       float64 `json:"score"`
	Suitability string  `json:"suitability"`
	Notes       string  `json:"notes"`
}

type ModelSuitabilityLedger struct {
	SchemaVersion string                `json:"schema_version"`
	Mode          string                `json:"mode"`
	Reviewers     []ReviewerSuitability `json:"reviewers"`
}
```

* **`suitability` values:** `schema_compliant_in_latest_run`, `usable_with_caution`, `not_schema_compliant`, `provider_unreliable`, `unknown`.

---

## 6. Agreement Diagnostics Design

We will compute lexical overlap diagnostics and output them to `consensus/agreement_diagnostics.json`.
For each pair of parseable reviewers, we calculate token Jaccard similarity for all cross-reviewer claim pairs and keep the top 5 closest pairs.

```go
type ClaimPair struct {
	ClaimAId         string  `json:"claim_a_id"`
	ClaimBId         string  `json:"claim_b_id"`
	ClaimANormalized string  `json:"claim_a_normalized"`
	ClaimBNormalized string  `json:"claim_b_normalized"`
	Jaccard          float64 `json:"jaccard"`
	BelowThreshold   bool    `json:"below_threshold"`
}

type PairwiseDiagnostic struct {
	ReviewerA         string      `json:"reviewer_a"`
	ReviewerB         string      `json:"reviewer_b"`
	Score             float64     `json:"score"`
	ClosestClaimPairs []ClaimPair `json:"closest_claim_pairs"`
}

type MatchingMethod struct {
	Type          string   `json:"type"`
	Normalization []string `json:"normalization"`
	Threshold     float64  `json:"threshold"`
}

type AgreementDiagnostics struct {
	SchemaVersion        string               `json:"schema_version"`
	AgreementStatus      string               `json:"agreement_status"`
	AgreementScore       float64              `json:"agreement_score"`
	ParseableReviewCount int                  `json:"parseable_review_count"`
	MatchingMethod       MatchingMethod       `json:"matching_method"`
	Explanation          string               `json:"explanation"`
	Pairwise             []PairwiseDiagnostic `json:"pairwise"`
	DiagnosticCategories []string             `json:"diagnostic_categories"`
}
```

---

## 7. Report Wording Changes

We will modify `renderReport` in [artifacts.go](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/simple/artifacts.go) to append:

```text
Real-provider status: degraded_but_usable.

Two reviewer responses parsed successfully under strict JSON validation.
One reviewer call failed at the provider/API layer.
This is a degraded candidate assessment, not a full triple-review result.

Agreement score is low or zero under lexical matching.
This does not prove the reviewers truly disagree semantically.
It means their extracted claims did not overlap enough under the current lexical/Jaccard rule.
Human review is required before drawing conclusions.
```

We will also output the **Model Operational Notes** section based on `ModelSuitabilityLedger` at the end of the report.

---

## 8. Artifact/Provenance Additions

The `Provenance` struct in [types.go](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/simple/types.go) will be extended to serialize the prompt hashes:
* `user_message_hash`
* `system_prompt_hash` (map per reviewer)
* `schema_example_hash`
* `temperature` (float64)
* `response_format` (string)

---

## 9. Test Plan

We will add a new test suite:
* `TestRealProviderStatus_FullTripleReviewReady`, `TestRealProviderStatus_DegradedButUsable`, `TestRealProviderStatus_DiagnosticOnly`, `TestRealProviderStatus_NotUsable`: Validates correctness of `determineRealProviderStatus`.
* `TestModelSuitabilityLedger_TwoParsedOneProviderFailed`: Simulates our latest smoke run and asserts correct suitability tags.
* `TestAgreementDiagnostics_ZeroAgreementPartialRun`: Asserts that `BuildAgreementDiagnostics` correctly evaluates Jaccard overlaps and populates pairwise logs.
* `TestReport_DegradedButUsableLanguage` & `TestReport_LowAgreementDoesNotClaimSemanticDisagreement`: Asserts that report contains exact disclaimers and required limitations.

---

## 10. Validation Plan

Validation commands to execute:
```bash
go test ./...
go test -race ./...
go vet ./...
```

---

## 11. Risks and Mitigations

* **Risk:** Extreme lexical drift. Jaccard similarity is strictly lexical, meaning synonym differences result in `0.000` similarity.
  * **Mitigation:** The warning disclaimers in `triple_review_report.md` will explicitly clarify that low lexical agreement is a matching artifact, not a proof of semantic disagreement.

---

## 12. Explicit Non-Goals

* We will **not** add embeddings or machine learning similarity models.
* We will **not** use TreeQuest in this mode.
* We will **not** change the lexical matching logic to allow fuzzy word matching (to keep it lexical-Jaccard based).

---

## 13. Acceptance Criteria

* `real_provider_status` is logged correctly as `degraded_but_usable` when 2 of 3 reviewers parse and 1 call fails.
* `consensus/model_suitability.json` is generated.
* `consensus/agreement_diagnostics.json` logs closest-pair Jaccard overlaps.
* Markdown report renders degraded notices and model operational notes.
* Prompt hashes, temperature, and response formats are stored.
* Unit, race, and vet tests pass successfully.

---

## 14. EBP/PTW Self-Audit

* This ticket accepts degraded real-provider smoke usability only.
* It does not prove the paper.
* It does not prove semantic convergence.
* It does not validate physics.
* It does not perform human faithfulness review.
* It does not promote EBP claims.
* It does not establish TreeQuest parity.
