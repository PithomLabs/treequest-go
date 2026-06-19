You are a senior Go architect, LLM-output-contract engineer, and EBP/PTW release reviewer.

Write a detailed implementation plan for the ticket:

# EBP-EVAL-SIMPLE-0003: Real-Provider Reviewer Schema Contract Hardening

If `EBP-EVAL-SIMPLE-0003` is already reserved for binary-boundary cleanup, rename this ticket to:

```text
EBP-EVAL-SIMPLE-0004: Real-Provider Reviewer Schema Contract Hardening
```

## Background

`EBP-EVAL-SIMPLE-0001` implemented `simple_triple_review`, where three independent LLM reviewers receive the same instruction:

```text
Apply EBP 2.1 on the attached physics paper.
```

The evaluator then parses their JSON responses, computes nine evaluator-owned metrics, compares agreement/disagreement, and emits a candidate-scoped artifact bundle.

The mock/e2e pipeline passed, but real-provider runs exposed a schema-contract problem.

Across several real-provider output bundles:

* Some reviewer API calls failed at provider/request level.
* Every successfully returned reviewer response failed strict parsing.
* Returned content was often useful-looking EBP prose/JSON, but structurally incompatible with the Go `ReviewerOutput` schema.
* Failures included:

  * `limitations` returned as a string instead of `[]string`;
  * `maps_identified`, `invariants_identified`, `toy_checks_identified`, `null_models_identified`, `obstructions_identified`, etc. returned as arrays of objects instead of arrays of strings;
  * `main_claims` returned with nonmatching keys such as `claim`, `notes`, `evidence_type`, or nested `debt` objects instead of the expected `claim_id`, `claim_text`, `evidence_quotes`, `ebp_debts`, and `status`;
  * null arrays or missing collections;
  * valid raw responses still scoring `0.0` because parse failed before scoring.

The suspected root cause is that the system prompt currently creates the schema/example using something like:

```go
schema, _ := json.Marshal(ReviewerOutput{})
```

Because `ReviewerOutput{}` has nil slices, the schema shown to the LLM contains fields like:

```json
"main_claims": null,
"maps_identified": null,
"limitations": null
```

This is not a useful JSON contract. It gives the LLM no concrete shape for nested claim objects or array-of-string fields.

## Goal

Create an implementation plan to harden the real-provider reviewer output contract while preserving strict evaluator behavior.

The goal is not to weaken the parser.

The goal is to make the prompt contract explicit enough that real LLMs can return parseable JSON matching the existing schema.

## Non-negotiable boundaries

Do not:

* modify `treequest-go`;
* use TreeQuest in `simple_triple_review`;
* weaken strict JSON validation to accept arbitrary unknown structures;
* accept LLM self-assigned scores;
* claim scientific truth;
* claim EBP promotion;
* claim human faithfulness review;
* claim source-faithful TreeQuest parity;
* hide parse failures as low-quality reviews.

Preserve:

* CLI-only local `.txt` / `.md` ingestion;
* no URL/PDF/arXiv/HTTP/pdftotext ingestion;
* prompt trust boundary;
* identical user message across reviewers;
* reviewer identity outside user content;
* strict first balanced JSON object extraction;
* `DisallowUnknownFields`;
* evaluator-computed scoring;
* artifact safety;
* no-faithfulness profile behavior;
* candidate-only report language;
* `treequest_used: false`.

## Required implementation areas

### 1. Replace zero-value schema with canonical populated example

Do not marshal `ReviewerOutput{}` directly for the system prompt.

Add a function such as:

```go
func ReviewerOutputExample(reviewerID, modelID string, debtIDs []string) ReviewerOutput
```

or:

```go
func reviewerOutputExample(c Config, reviewerID, modelID string) ReviewerOutput
```

The example must populate every required field with a valid placeholder.

It must ensure all slices are non-nil and shown as arrays, never `null`.

The example should include:

```json
{
  "reviewer_id": "reviewer_1",
  "model_id": "example-model",
  "paper_summary": "Brief 2-3 sentence summary of the paper.",
  "main_claims": [
    {
      "claim_id": "claim_1",
      "claim_text": "Precise candidate claim extracted from the paper.",
      "evidence_quotes": [
        {
          "quote": "Exact quote copied from the paper.",
          "section_hint": "Abstract or Section 1"
        }
      ],
      "ebp_debts": ["needMap", "needInvariant"],
      "status": "candidate_unreviewed"
    }
  ],
  "maps_identified": [
    "Map from proposed mathematical structure to observable physical quantity."
  ],
  "invariants_identified": [
    "Claimed invariant, conservation law, symmetry, or preserved quantity."
  ],
  "toy_checks_identified": [
    "Suggested finite toy check or calculation."
  ],
  "null_models_identified": [
    "Alternative baseline explanation or simpler rival model."
  ],
  "obstructions_identified": [
    "Technical blocker, missing derivation, or circularity risk."
  ],
  "faithfulness_limits": [
    "Human faithfulness review was not performed."
  ],
  "overclaim_warnings": [
    "Do not treat this as proof or EBP promotion."
  ],
  "recommended_next_steps": [
    "Extract exact equations and map them to EBP debts."
  ],
  "overall_assessment": "Candidate EBP assessment only.",
  "limitations": [
    "Automated candidate assessment only.",
    "Human faithfulness review was not performed.",
    "No EBP promotion is claimed."
  ]
}
```

The actual debt examples should come from the loaded policy when possible. For non-EBP policies, use that policy’s debt IDs instead of hardcoded EBP debt IDs.

### 2. Add explicit schema contract text to the system prompt

The system prompt must say, plainly:

```text
Return exactly one JSON object.
Do not use Markdown fences.
Do not return prose before or after JSON.
Do not add unknown fields.
Do not rename keys.
All array fields must be arrays, never null.
Use [] when there are no items.
```

Explicitly list fields that must be arrays of strings:

```text
maps_identified
invariants_identified
toy_checks_identified
null_models_identified
obstructions_identified
faithfulness_limits
overclaim_warnings
recommended_next_steps
limitations
```

Explicitly say:

```text
Do not return arrays of objects for those fields.
Flatten rich details into concise strings.
```

Explicitly define `main_claims`:

```text
main_claims must be an array of objects with exactly:
claim_id
claim_text
evidence_quotes
ebp_debts
status
```

Explicitly define `evidence_quotes`:

```text
evidence_quotes must be an array of objects with exactly:
quote
section_hint
```

Explicitly define `ebp_debts`:

```text
ebp_debts must be an array of strings using debt IDs from the active policy.
Do not return a debt object or map.
```

### 3. Keep strict parsing

Preserve:

```go
jsonutil.ExtractFirstJSONObject
json.Decoder.DisallowUnknownFields
required-field validation
identity validation
limitation validation
policy-debt validation
```

Do not add broad fallback parsing that silently accepts wrong structures.

The plan should explicitly reject the quick fix of decoding into `map[string]any` and trying to accept everything.

### 4. Improve parseability-aware run status

Add a clearer distinction between:

```text
call completion
parse completion
assessment availability
```

For example:

```json
{
  "call_run_status": "triple_review_complete",
  "parse_run_status": "no_parseable_reviews",
  "assessment_status": "assessment_unavailable_schema_parse_failed",
  "returned_response_count": 3,
  "parseable_review_count": 0
}
```

Recommended statuses:

```text
parse_run_status:
  all_reviews_parseable
  partial_reviews_parseable
  one_review_parseable
  no_parseable_reviews

assessment_status:
  candidate_assessment_available
  degraded_candidate_assessment_available
  assessment_unavailable_schema_parse_failed
  run_failed_no_reviewer_content
```

If three reviewers return raw content but none parse:

* preserve raw files;
* preserve error files;
* produce diagnostic artifacts if appropriate;
* do not present `final_score` or agreement as meaningful EBP assessment;
* report must say no parseable reviewer outputs were available.

### 5. Improve report wording for parse failures

If `parseable_review_count == 0`, the report must state:

```text
All reviewers returned unparseable schema-incompatible output.
No reviewer score, agreement score, or EBP assessment should be interpreted as meaningful.
This is a schema-contract failure, not an EBP judgment on the paper.
```

If some parse and some fail:

```text
This is a degraded candidate assessment. Agreement and scoring are based only on parseable reviewer outputs.
```

### 6. Add observed-failure fixtures

Create test fixtures based on the observed real-provider failure classes:

```text
testdata/reviewer_outputs/limitations_as_string.json
testdata/reviewer_outputs/maps_as_objects.json
testdata/reviewer_outputs/wrong_claim_keys.json
testdata/reviewer_outputs/debts_as_object.json
testdata/reviewer_outputs/null_collections.json
```

These should remain invalid under strict parsing.

The point is to ensure failures are clear and produce correct diagnostic statuses, not to accept malformed structures.

### 7. Add schema-example tests

Required tests:

```text
TestReviewerOutputExample_NoNullArrays
TestReviewerOutputExample_ContainsMainClaimShape
TestReviewerOutputExample_StringArrayFieldsAreArraysOfStrings
TestReviewerOutputExample_UsesPolicyDebtIDs
TestSystemPrompt_IncludesExplicitArrayNeverNullRule
TestSystemPrompt_IncludesNoUnknownFieldsRule
TestSystemPrompt_IncludesNoMarkdownFenceRule
```

### 8. Add real-provider parse-failure regression tests

Required tests:

```text
TestReviewerParse_ObservedLimitationsAsStringFailsClearly
TestReviewerParse_ObservedMapsAsObjectsFailsClearly
TestReviewerParse_ObservedWrongClaimKeysFailsClearly
TestReviewerParse_ObservedDebtsAsObjectFailsClearly
TestReviewerParse_ObservedNullCollectionsFailsClearly
TestTripleReview_AllReturnedButNoneParseable_StatusIsNoParseableReviews
TestTripleReview_ReportDoesNotTreatAllZeroParseAsEBPScore
TestTripleReview_PartialParseableReviews_StatusIsDegradedAssessment
```

### 9. Provider JSON-mode support where available

If OpenRouter/model supports JSON mode or response format, add optional support such as:

```json
{
  "response_format": {
    "type": "json_object"
  }
}
```

But do not rely on JSON mode alone.

The prompt schema contract and strict parser remain authoritative.

If the current `llm.GenerateRequest` does not support response format, propose a minimal extension:

```go
type GenerateRequest struct {
    Role           string
    Task           string
    System         string
    User           string
    Model          string
    Temperature    float64
    MaxTokens      int
    ResponseFormat string // "", "json_object"
    Metadata       map[string]string
}
```

Or use a provider-specific option struct if already present.

### 10. Lower reviewer temperature

For real-provider schema compliance, set default triple-review temperature to:

```text
0.0 or 0.1
```

Make it configurable only if needed.

This is a schema-following task, not a creativity task.

### 11. Optional schema repair path

Do not implement automatic repair unless explicitly enabled.

If included in the plan, put it behind a flag:

```text
--repair-malformed-json
```

Repair must be explicit and recorded:

```json
{
  "repair_attempted": true,
  "repair_succeeded": true,
  "original_parse_status": "review_parse_failed",
  "final_parse_status": "review_parse_repaired"
}
```

Repair prompt rule:

```text
Convert the raw response into the exact JSON schema.
Do not add new claims.
Do not change scientific meaning.
Only reformat.
Use [] or empty strings for missing fields.
```

For this ticket, prefer leaving repair disabled by default.

### 12. Real-provider smoke rerun criteria

After implementing the schema-contract repair, rerun a small real-provider smoke test with three selected models.

The acceptance target should not require perfect scientific quality.

It should require:

```text
at least 2 of 3 returned responses parse successfully
parseable_review_count >= 2
bundle generated
agreement metrics available
no secret leakage
no proof/promotion language
faithfulness remains not_assessed
```

If free-tier models remain unreliable, report model/provider unreliability separately from evaluator failure.

## Required implementation-plan structure

Return the implementation plan in this exact structure:

1. Verdict and scope
2. Root-cause summary
3. Files/functions likely affected
4. Schema example design
5. System prompt contract changes
6. Strict parsing preservation
7. Parseability-aware run-status design
8. Report/artifact changes
9. Provider JSON-mode and temperature changes
10. Optional repair mode decision
11. Test plan
12. Real-provider smoke-test plan
13. Risks and mitigations
14. Explicit non-goals
15. Acceptance criteria
16. EBP/PTW self-audit

## Acceptance criteria

The repair is accepted when:

* system prompt no longer shows `null` arrays for schema fields;
* canonical JSON example shows every required field and nested structure;
* all arrays are shown as arrays;
* prompt explicitly forbids Markdown fences, unknown fields, renamed keys, null arrays, and object arrays for string-list fields;
* strict parsing remains enabled;
* observed malformed outputs fail clearly;
* all-returned-but-none-parseable run is classified as schema-contract failure, not EBP assessment;
* partial-parse runs are marked degraded;
* no report presents all-zero parse failures as meaningful EBP scores;
* JSON mode is requested where supported or a clear TODO is documented;
* reviewer temperature is low for schema compliance;
* tests pass:

  * `go test ./...`
  * `go test -race ./...`
  * `go vet ./...`
* mock EBP and non-EBP runs still pass;
* real-provider smoke test produces at least 2 parseable reviewer outputs, or clearly diagnoses provider/model schema noncompliance without hiding it.

## EBP/PTW self-audit requirements

Classify:

* needMap
* needInvariant
* needToyCheck
* needNullModel
* needObstruction
* needFaithfulnessReview
* promotion status

Use strict language:

* This ticket repairs the reviewer-output schema contract.
* It does not prove any paper claim.
* It does not validate the physics.
* It does not perform human faithfulness review.
* It does not promote EBP claims.
* It does not establish TreeQuest parity.
* Real-provider parseability is a reviewer-contract metric, not a truth metric.

## Review standard

Be implementation-specific.

Give concrete Go files, functions, tests, status fields, artifact files, and report wording.

Preserve strictness.

Do not solve schema failures by making the evaluator permissive.

The correct philosophy is:

```text
Make the contract clearer.
Keep the gate strict.
Make failures diagnostically honest.
```
