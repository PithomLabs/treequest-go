## Verdict

```text
accept_with_minor_repairs
```

The implementation plan is directionally correct and addresses the real-provider failure mode: the LLMs were not failing primarily because they “do not know physics,” but because the prompt gave them a weak JSON contract with `null` arrays and no concrete nested examples. The proposed repair—replace `json.Marshal(ReviewerOutput{})` with a fully populated canonical example, add explicit schema-contract language, preserve strict parsing, and make parseability status visible—is the right mitigation. 

## What the plan gets right

The plan correctly identifies the main root cause:

```go
schema, _ := json.Marshal(ReviewerOutput{})
```

Because nil Go slices marshal as `null`, the LLM saw a schema shape like:

```json
"main_claims": null,
"maps_identified": null,
"limitations": null
```

That invited the model to invent its own richer structures. The plan’s `reviewerOutputExample(...)` fix is the right answer.

It also correctly preserves the important invariant:

```text
Make the schema contract clearer.
Keep strict parsing.
Do not silently accept malformed structures.
```

That is very EBP-safe.

## Minor repairs before implementation

### 1. Do not add `--repair-malformed-json` yet

The plan says to design the flag but keep it disabled by default. I would go stricter:

```text
Do not implement repair mode in this ticket.
```

Reason: this ticket should prove the **first-pass schema contract** works. A repair path can hide whether the reviewer prompt is actually fixed. Add repair later only after clean parseability is measured.

### 2. Avoid hardcoded EBP fallback debt IDs in examples

The plan has:

```go
if len(debtIDs) == 0 {
    debtIDs = []string{"needMap", "needInvariant"}
}
```

Better:

```text
If the active policy has no debt IDs, use generic placeholders like "exampleDebt1", "exampleDebt2", or fail the policy/profile validation earlier.
```

Otherwise a non-EBP policy with a malformed/empty debt list could accidentally receive EBP-looking examples.

### 3. Make JSON mode optional per provider/model

The plan adds `ResponseFormat: "json_object"`, which is good, but some OpenRouter models/providers may reject it. So the implementation should treat JSON mode as:

```text
requested_when_supported_or_configured
```

not universal hard requirement.

Add fallback behavior:

```text
If provider rejects response_format, record reviewer_call_failed with provider_response_format_unsupported, or retry once without JSON mode only if explicitly configured.
```

For v0.1, I prefer no silent retry. Make the failure visible.

### 4. Rename status fields carefully

The plan proposes adding status fields to `ScoringSummary`. That is reasonable, but I would avoid overloading “scoring” with run lifecycle.

Better structure:

```go
type RunStatusSummary struct {
    CallRunStatus         string `json:"call_run_status"`
    ParseRunStatus        string `json:"parse_run_status"`
    AssessmentStatus      string `json:"assessment_status"`
    ReturnedResponseCount int    `json:"returned_response_count"`
    ParseableReviewCount  int    `json:"parseable_review_count"`
}
```

Then `ScoringSummary` can include or reference it. This keeps score semantics separate from run-completeness semantics.

### 5. Real-provider smoke success should be softer

The plan says success requires at least 2 of 3 parse successfully. That is a good target, but free-tier models can be flaky. Acceptance should distinguish:

```text
implementation acceptance
```

from:

```text
provider/model suitability
```

So the acceptance criterion should be:

```text
The code correctly requests the stricter schema and reports parseability honestly. A real-provider smoke run should aim for ≥2 parseable responses; if not achieved, the report must clearly classify whether failures are provider call failures, schema noncompliance, or evaluator defects.
```

## Recommended implementation priority

```text
1. Add canonical populated ReviewerOutput example.
2. Add explicit schema-contract prompt text.
3. Add tests proving no null arrays and correct nested shape.
4. Add observed malformed-output fixtures that still fail clearly.
5. Add parseability-aware run/report status.
6. Add low temperature.
7. Add optional JSON response format support, carefully.
8. Re-run mock EBP/non-EBP.
9. Run real-provider smoke test.
```

## Updated ticket status

```text
EBP-EVAL-SIMPLE-0004 plan: accepted_with_minor_repairs
ready_for_implementation_after_minor_scope_tightening
```

## EBP/PTW self-audit

**needMap:** Satisfied. The plan maps schema prompt → reviewer JSON → strict parser → scoring/reporting.

**needInvariant:** Satisfied if strict parsing remains and repair mode is not silently enabled.

**needToyCheck:** Satisfied by proposed malformed-output fixtures and no-null-schema tests.

**needNullModel:** Compare old zero-value schema against new populated example on the same real-provider models.

**needObstruction:** Main obstruction is LLM schema drift, not physics competence.

**needFaithfulnessReview:** Still `not_assessed`.

**Promotion status:** `schema_contract_hardening_plan_accepted`; no physics truth, EBP promotion, or human faithfulness review claimed.
