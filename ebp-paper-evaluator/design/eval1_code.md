# Walkthrough - EBP-EVAL-SIMPLE-0004: Real-Provider Reviewer Schema Contract Hardening

We have successfully implemented the reviewer schema contract hardening changes in `ebp-paper-evaluator`. All strict JSON validation gates are preserved, while the prompt contract is now robust enough for real-world LLMs to comply with the Go JSON structs.

## Changes Made

### 1. Extracted and Configured Populated Example JSON
* In [triple_review.go](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/simple/triple_review.go), implemented `reviewerOutputExample` which dynamically constructs a fully-populated mock `ReviewerOutput` struct (never serializing empty slice fields to `null`). It extracts active policy debt IDs from `c.Policy.IR.DebtItems` (falling back to generic `"exampleDebt1"`, `"exampleDebt2"` placeholders if none exist).
* Updated `systemPrompt` to serialize the populated example struct to inject in the `OUTPUT_SCHEMA_SHAPE` block of the system prompt.

### 2. Added Explicit Prompt Directives
* Added explicit type contracts to `systemPrompt` instructing the LLM:
  * To return exactly one JSON object without Markdown fences or wrapping prose.
  * To never return `null` for array fields and use `[]` instead.
  * Explicitly listed the 9 fields that must be flat `[]string` arrays, forbidding object arrays.
  * Explicitly defined the schema structure for `main_claims`, `evidence_quotes`, and `ebp_debts` arrays.

### 3. Parseability-Aware Run Status and Structs
* Added `RunStatusSummary` struct in [types.go](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/simple/types.go) and embedded it inside `ScoringSummary` to separate score metrics from run lifecycle metrics.
* Updated `summarize` in [triple_review.go](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/simple/triple_review.go) to compute:
  * `call_run_status` (e.g. `triple_review_complete`, `partial_triple_review_two_reviewers`)
  * `parse_run_status` (e.g. `all_reviews_parseable`, `no_parseable_reviews`)
  * `assessment_status` (e.g. `candidate_assessment_available`, `assessment_unavailable_schema_parse_failed`)
  * `returned_response_count` and `parseable_review_count`

### 4. Configured Temperature & Response Format
* Forced triple-review model calls to use a low temperature (`0.1`) inside `reviewOne` to improve instruction following.
* Proposed the `ResponseFormat` extension in `GenerateRequest` ([client.go](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/llm/client.go)), which sets OpenRouter's `response_format` to `json_object` dynamically.
* If a provider rejects the `response_format` JSON parameter, `sanitizeError` maps it to `provider_response_format_unsupported` to trigger a visible error without silent retries.

### 5. Updated Reports
* Modified `renderReport` in [artifacts.go](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/simple/artifacts.go) to output clear warning blocks when the run has no parseable reviews:
  > All reviewers returned unparseable schema-incompatible output.
  > No reviewer score, agreement score, or EBP assessment should be interpreted as meaningful.
  > This is a schema-contract failure, not an EBP judgment on the paper.
* Outputs a degraded candidate assessment disclaimer when only a subset of reviewer responses successfully parse.

### 6. Created Real-Provider Failure Fixtures
Created 5 JSON fixtures under `testdata/reviewer_outputs/` representing observed real-provider failure classes:
* [limitations_as_string.json](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/testdata/reviewer_outputs/limitations_as_string.json)
* [maps_as_objects.json](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/testdata/reviewer_outputs/maps_as_objects.json)
* [wrong_claim_keys.json](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/testdata/reviewer_outputs/wrong_claim_keys.json)
* [debts_as_object.json](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/testdata/reviewer_outputs/debts_as_object.json)
* [null_collections.json](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/testdata/reviewer_outputs/null_collections.json)

---

## Verification and Tests

All tests passed successfully:

1. **Schema-Example Tests:**
   * `TestReviewerOutputExample_NoNullArrays`: Verified example JSON contains no `:null` arrays.
   * `TestReviewerOutputExample_ContainsMainClaimShape`: Checked that mock claims populate correct keys.
   * `TestReviewerOutputExample_StringArrayFieldsAreArraysOfStrings`: Validated maps and limitations marshal to string lists.
   * `TestReviewerOutputExample_UsesPolicyDebtIDs`: Verified example debts map to active policy.
   * `TestSystemPrompt_IncludesExplicitArrayNeverNullRule`, `TestSystemPrompt_IncludesNoUnknownFieldsRule`, `TestSystemPrompt_IncludesNoMarkdownFenceRule`: Checked prompt compliance.
2. **Regression Parse Tests:**
   * Verified that all five observed malformed JSON fixtures fail strict parsing as expected (e.g. `TestReviewerParse_ObservedLimitationsAsStringFailsClearly`).
   * `TestTripleReview_AllReturnedButNoneParseable_StatusIsNoParseableReviews`: Asserts correct status transition.
   * `TestTripleReview_ReportDoesNotTreatAllZeroParseAsEBPScore`: Validates report warning block generation.
   * `TestTripleReview_PartialParseableReviews_StatusIsDegradedAssessment`: Asserts degraded status transition.

### Validation Commands Run
```bash
go test ./...
go test -race ./...
go vet ./...
```

**Results:**
* `go test ./...` — **PASS**
* `go test -race ./...` — **PASS**
* `go vet ./...` — **PASS**

---

## EBP/PTW Self-Audit

* **Type:** Reviewer-Output Schema Contract Hardening.
* **Limitations:**
  * This implementation plan repairs the reviewer-output schema contract.
  * It does not prove any paper claim.
  * It does not validate the physics.
  * It does not perform human faithfulness review.
  * It does not promote EBP claims.
  * It does not establish TreeQuest parity.
  * Real-provider parseability is a reviewer-contract metric, not a truth metric.
* **Status:** Complete.

- `[x]` Add RunStatusSummary and update Result/ScoringSummary in [types.go](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/simple/types.go)
- `[x]` Implement GenerateRequest extension for ResponseFormat in [client.go](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/llm/client.go)
- `[x]` Implement reviewerOutputExample and update systemPrompt in [triple_review.go](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/simple/triple_review.go)
- `[x]` Add observed malformed-output JSON fixtures in testdata/reviewer_outputs/
- `[x]` Add schema-example and regression tests in [triple_review_test.go](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/simple/triple_review_test.go)
- `[x]` Update Run and reviewOne in [triple_review.go](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/simple/triple_review.go) to set ResponseFormat, low temperature, and populate RunStatusSummary
- `[x]` Update renderReport in [artifacts.go](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/simple/artifacts.go) to format warning statements
- `[x]` Run validation tests: `go test ./...`, `go test -race ./...`, and `go vet ./...`
- `[x]` Run real-provider smoke test using CLI
