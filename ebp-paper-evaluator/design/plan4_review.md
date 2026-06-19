This is a good implementation plan. I would mark it:

```text
accept
```

It preserves the simplified product philosophy: **three independent LLMs apply EBP 2.1 to the same local physics paper, then the evaluator scores and compares their responses.** It correctly keeps TreeQuest out of scope.

## What is especially strong

The plan gets the key architectural boundaries right:

```text
PASS: no TreeQuest APIs
PASS: no treequest-go modification
PASS: local .txt/.md ingestion reused
PASS: three independent concurrent reviewers
PASS: identical user message across reviewers
PASS: reviewer identity kept out of user content
PASS: strict JSON extraction reused
PASS: malformed/failing reviewers preserved as explicit partial results
PASS: deterministic artifact ordering by reviewer_id
PASS: equal-weight nine-metric final_score
PASS: lexical agreement only, no overclaiming semantic understanding
PASS: treequest_used: false in provenance
```

The most important design choice is this one:

```text
Execution is concurrent.
Artifacts are deterministic.
Reviewers are independent.
The final report is candidate-scoped.
```

That is exactly right.

## One refinement I strongly recommend

Add a top-level distinction between:

```text
reviewer_score
```

and

```text
run_completeness
```

Because a provider timeout should not be confused with a bad EBP review.

Example:

```json
{
  "reviewer_id": "reviewer_2",
  "call_status": "reviewer_call_failed",
  "reviewer_score": 0.0,
  "failure_affects": "run_completeness_not_review_quality"
}
```

Then at run level:

```json
{
  "run_status": "partial_triple_review_two_reviewers",
  "run_completeness": 0.67,
  "mean_reviewer_score_parseable_only": 0.74,
  "mean_reviewer_score_with_failures": 0.49
}
```

This avoids misleading users when one API call fails.

## Second refinement

The `claim_coverage` metric depends on the “combined parseable claim clusters.” That is fine for v0.1, but label it carefully:

```text
relative_claim_coverage
```

because it only measures coverage relative to what the three reviewers found, not coverage against the actual paper truth.

So:

```text
claim_coverage = relative claim coverage among parseable reviewers
```

not:

```text
claim_coverage = all important claims in the paper
```

That distinction matters.

## Third refinement

For `source_grounding`, require quotes to be exact normalized-paper matches, but do not over-penalize if a reviewer gives section hints without exact quote text. Better:

```text
source_grounding = exact_quote_supported_claims / total_claims
source_grounding_notes = section_hint_only_claims
```

So section hints are visible but do not count as full grounding.

## Recommended status language

Use this:

```text
EBP-EVAL-SIMPLE-0001 plan accepted.

Implement simple_triple_review as an independent concurrent three-reviewer mode in ebp-paper-evaluator only.

TreeQuest is not used.
treequest-go is not modified.
The evaluator scores LLM review quality and agreement under EBP-style metrics.
It does not score physics truth, perform human faithfulness review, or promote paper claims.
```

## Acceptance criteria to keep locked

The implementation should not be accepted unless these are true:

```text
go test ./... PASS
go test -race ./... PASS
go vet ./... PASS
mock EBP triple review PASS
mock non-EBP triple review PASS
pkg/simple has no TreeQuest import PASS
provenance treequest_used:false PASS
0-reviewer-return case does not emit completed assessment bundle PASS
all reviewer artifacts sorted deterministically PASS
```

## EBP/PTW self-audit

**needMap:** Satisfied. The plan maps paper + EBP policy + three LLM responses → scoring → agreement/disagreement → artifact bundle.

**needInvariant:** Satisfied. TreeQuest boundary, local-only ingestion, identical reviewer prompt, deterministic artifacts, no-promotion language.

**needToyCheck:** Specified through mock EBP and non-EBP triple-review runs plus 3/2/1/0 reviewer completion tests.

**needNullModel:** Partially satisfied. The non-EBP policy fixture remains; later you may add a single-review baseline to compare whether triple review gives more useful disagreement visibility.

**needObstruction:** Main risks are shallow lexical matching, false consensus, provider failures, and inflated claim coverage. Your plan mitigates these by using lexical-only claims, partial status, and explicit limitations.

**needFaithfulnessReview:** Still `not_assessed`.

**Promotion status:** `implementation_plan_accepted_for_simple_triple_review`; no scientific truth, EBP promotion, human faithfulness review, or TreeQuest parity claim.
