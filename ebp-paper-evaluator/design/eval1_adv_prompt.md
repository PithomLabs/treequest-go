Here is the prompt for `EBP-EVAL-SIMPLE-0005`.

You are an adversarial senior Go reviewer, real-provider LLM integration auditor, and EBP/PTW artifact auditor.

Review the real-provider smoke-test output after:

# EBP-EVAL-SIMPLE-0004: Real-Provider Reviewer Schema Contract Hardening

This ticket should be treated as:

# EBP-EVAL-SIMPLE-0005: Real-Provider Smoke Artifact Review

## Background

`EBP-EVAL-SIMPLE-0004` reportedly repaired the reviewer schema contract for `simple_triple_review` by:

* replacing the zero-value `ReviewerOutput{}` schema with a populated `reviewerOutputExample`;
* ensuring arrays are shown as arrays, not `null`;
* using active policy debt IDs or generic placeholders;
* adding explicit prompt rules forbidding Markdown fences, prose wrappers, unknown keys, renamed keys, null arrays, object arrays for string-list fields, and debt maps;
* preserving strict first-object JSON extraction and `DisallowUnknownFields`;
* adding `RunStatusSummary`;
* separating:

  * `call_run_status`
  * `parse_run_status`
  * `assessment_status`
  * `returned_response_count`
  * `parseable_review_count`
* setting low reviewer temperature, e.g. `0.1`;
* adding OpenRouter `response_format: json_object` support where applicable;
* marking unsupported JSON response format as `provider_response_format_unsupported`;
* adding report warnings when no parseable reviews exist;
* adding observed malformed-output fixtures and tests.

Reported validation:

```bash
go test ./...
go test -race ./...
go vet ./...
```

all pass.

A real-provider smoke test was reportedly run, but the parseable count, per-reviewer statuses, model behavior, bundle path, and final artifact quality still need review.

## Review objective

Determine whether the real-provider smoke run is acceptable for current v0.1 operational confidence.

This review does **not** prove the physics paper true.

It only evaluates whether real models can produce parseable, artifact-safe, candidate-scoped EBP-style reviews under the hardened schema contract.

Return one verdict:

```text
accept
accept_with_repairs
reject_for_now
```

## Required input to inspect

Inspect the real-provider smoke-test output bundle.

Expected bundle path should be supplied by the implementer, for example:

```text
/tmp/ebp-eval-simple-triple-review-real-provider-smoke
```

or a timestamped path under:

```text
/home/chaschel/Desktop/physics/sakana/out
```

If multiple smoke runs exist, review the latest run and briefly compare it to earlier failed runs.

## Non-negotiable boundaries

The smoke artifact is acceptable only if:

1. `simple_triple_review` remains local `.txt` / `.md` only.
2. TreeQuest is not used.
3. Provenance records:

```json
{
  "mode": "simple_triple_review",
  "treequest_used": false
}
```

4. Prompt ledger shows `response_format` or equivalent JSON-mode metadata when requested.
5. Prompt ledger shows low temperature, e.g. `0.1`, if recorded.
6. All reviewer user messages are identical.
7. Reviewer identity is absent from user content.
8. Reviewer identity, model ID, role/task, schema, and safety rules are in system/metadata.
9. Paper content is escaped and wrapped in `<untrusted_paper>`.
10. No paper instruction overrides the system/policy contract.
11. Strict JSON parsing remains in effect.
12. LLM self-scores are ignored.
13. Evaluator-computed scores are bounded `[0.0, 1.0]`.
14. Review quality and run completeness are separate.
15. Parseability-aware statuses are present.
16. No report claims proof, truth, EBP promotion, human faithfulness review, or TreeQuest parity.
17. Faithfulness remains `not_assessed`.
18. No secrets, API keys, Authorization headers, Bearer tokens, request headers, or env values appear in artifacts.

## Success targets

The strongest success condition is:

```text
returned_response_count = 3
parseable_review_count >= 2
assessment_status = candidate_assessment_available
or
assessment_status = degraded_candidate_assessment_available
```

Acceptable weaker outcome:

```text
returned_response_count >= 2
parseable_review_count >= 1
assessment_status = degraded_candidate_assessment_available
```

This weaker outcome may be accepted only if failures are clearly attributable to provider/model reliability and the bundle reports degraded status honestly.

Reject for now if:

```text
parseable_review_count = 0
```

unless the objective was only to test diagnostic handling. In that case, classify it as:

```text
schema_diagnostics_working_but_real_provider_operational_readiness_not_established
```

## Required audit areas

### A. Provenance and run-status audit

Inspect:

```text
run/provenance.json
consensus/scoring_summary.json
report/triple_review_report.md
reviews/reviewer_1_score.json
reviews/reviewer_2_score.json
reviews/reviewer_3_score.json
```

Extract and report:

```json
{
  "mode": "...",
  "treequest_used": false,
  "call_run_status": "...",
  "parse_run_status": "...",
  "assessment_status": "...",
  "returned_response_count": 0,
  "parseable_review_count": 0,
  "models": {
    "reviewer_1": "...",
    "reviewer_2": "...",
    "reviewer_3": "..."
  }
}
```

Classify status as:

```text
real_provider_operationally_usable
degraded_but_usable
diagnostic_only
not_usable
```

### B. Per-reviewer outcome audit

For each reviewer, inspect:

```text
reviews/reviewer_N_raw.txt
reviews/reviewer_N_parsed.json
reviews/reviewer_N_score.json
```

or, if failed:

```text
reviews/reviewer_N_error.json
```

Report for each reviewer:

```json
{
  "reviewer_id": "reviewer_1",
  "model_id": "...",
  "call_status": "...",
  "parse_status": "...",
  "score": 0.0,
  "failure_category": "none | provider_call_failed | provider_response_format_unsupported | review_parse_failed | schema_incompatible_output | timeout | unknown"
}
```

For parse failures, identify the exact schema mismatch:

* string where `[]string` expected;
* object array where `[]string` expected;
* missing required field;
* renamed key;
* unknown field;
* null collection;
* malformed JSON;
* markdown/prose wrapper;
* wrong reviewer/model ID;
* limitation language missing;
* unsupported debt ID;
* other.

### C. Schema-contract success audit

Inspect raw successful reviewer outputs.

Determine whether the new prompt contract worked:

* arrays are arrays, not `null`;
* `main_claims` uses expected keys;
* `evidence_quotes` uses `quote` and `section_hint`;
* `ebp_debts` is a flat array of strings;
* string-list fields are arrays of strings;
* no arrays of objects for `maps_identified`, etc.;
* no unknown fields;
* no Markdown fences;
* no prose before or after JSON;
* no LLM-provided score field;
* no proof/promotion language.

Classify:

```text
schema_contract_success
schema_contract_partial_success
schema_contract_failure
```

### D. Prompt-ledger audit

Inspect:

```text
run/prompt_ledger.json
```

Verify:

* all three user messages are byte-identical or hash-identical;
* user message begins with:

```text
Apply EBP 2.1 on the attached physics paper.
```

* `<untrusted_paper>` wrapper is present;
* paper content is escaped;
* reviewer identity is absent from user content;
* reviewer identity appears in system/metadata;
* system prompt includes populated `OUTPUT_SCHEMA_SHAPE`;
* system prompt includes:

  * arrays never null;
  * no Markdown fences;
  * no unknown fields;
  * no renamed keys;
  * no object arrays for string-list fields;
  * debt IDs must be flat string arrays;
* prompt records include model ID, role/task, document hash, policy hash, profile ID;
* response_format/json mode is recorded if supported;
* low temperature is recorded if prompt ledger stores request settings.

### E. Scoring and report audit

Inspect:

```text
consensus/scoring_summary.json
report/triple_review_report.md
```

Confirm:

* all scores are evaluator-computed;
* final score is equal-weight mean of nine metrics;
* malformed/failed reviewers have score `0.0`;
* failed reviewer score is not confused with review quality;
* run completeness is separate from review quality;
* if `parseable_review_count == 0`, report does not present all-zero scores as meaningful;
* if partial parseability, report clearly says degraded candidate assessment;
* agreement is computed only from parseable reviews;
* cross-model agreement is `0.0` if fewer than two parseable reviews exist;
* lexical agreement limitation is stated;
* relative claim coverage is not represented as paper-truth coverage.

### F. Artifact completeness audit

Verify the bundle includes:

```text
source/
policy/
reviews/
consensus/
report/
run/
```

Expected files:

```text
source/source_ref.json
source/source_hash.txt
policy/policy_snapshot.md
policy/policy_ir.json
policy/policy_hash.txt
reviews/reviewer_1_raw.txt or reviews/reviewer_1_error.json
reviews/reviewer_1_score.json
reviews/reviewer_2_raw.txt or reviews/reviewer_2_error.json
reviews/reviewer_2_score.json
reviews/reviewer_3_raw.txt or reviews/reviewer_3_error.json
reviews/reviewer_3_score.json
consensus/agreement_ledger.json
consensus/disagreement_ledger.json
consensus/combined_claims.json
consensus/scoring_summary.json
report/triple_review_report.md
run/provenance.json
run/budget_usage.json
run/prompt_ledger.json
```

Check hash consistency:

* source hash matches document/provenance reference;
* policy hash matches policy snapshot;
* prompt hashes are present;
* model mapping is present;
* budget records exist per reviewer.

### G. Secret and privacy audit

Search artifacts for:

```text
OPENROUTER_API_KEY
Authorization
Bearer
sk-
x-api-key
.env
provider request headers
raw local absolute paths unless --include-local-paths was enabled
```

Reject if any real credential, header, or secret value is stored.

The string literal `OPENROUTER_API_KEY` is acceptable only in source code, not in generated artifacts unless it is part of a documented placeholder and not a value.

### H. Candidate-scope and EBP/PTW language audit

Reject if the report or parsed reviewer outputs claim:

```text
proved
solved
validated physics
final truth
full EBP promotion
human faithfulness completed
source-faithful TreeQuest parity
```

Negative limitation sentences are allowed.

Required report language:

```text
This is an automated candidate EBP assessment.
It is not a proof of the paper's claims.
It is not full EBP promotion.
Human faithfulness review was not performed.
The three LLM reviews are comparison signals, not authorities.
```

### I. Model suitability audit

Based on the smoke run, classify each model:

```text
usable_for_schema_contract
usable_with_caution
not_schema_compliant
provider_unreliable
unknown
```

This is not a physics-truth rating.

It is only a schema-compliance and operational reliability rating for this evaluator mode.

### J. Regression comparison to pre-hardening runs

If earlier failed smoke outputs are available, briefly compare:

* pre-hardening parseable count;
* post-hardening parseable count;
* main failure modes before;
* main failure modes after;
* whether the populated schema and prompt rules improved parseability.

## Required validation commands

If reviewing from source, run:

```bash
go test ./...
go test -race ./...
go vet ./...
```

Optionally rerun smoke test:

```bash
ebp-paper-evaluator triple-review \
  --paper ./testdata/papers/local_physics_paper.txt \
  --policy ./policies/ebp_v2_1.md \
  --profile automated-no-faithfulness \
  --models MODEL_A,MODEL_B,MODEL_C \
  --out /tmp/ebp-eval-simple-triple-review-real-provider-smoke
```

Do not print or store API keys.

## Required output format

Return the review in this exact structure:

1. Verdict

   * `accept`, `accept_with_repairs`, or `reject_for_now`
   * one paragraph rationale

2. Real-provider run summary

   * bundle path
   * models
   * returned_response_count
   * parseable_review_count
   * call_run_status
   * parse_run_status
   * assessment_status

3. Per-reviewer table
   Columns:

   * reviewer
   * model
   * call_status
   * parse_status
   * score
   * failure_category
   * usable_for_schema_contract?

4. Critical blockers

5. High-priority repairs

6. Medium-priority improvements

7. Schema-contract audit

8. Prompt-ledger audit

9. Scoring/report audit

10. Artifact/secret/privacy audit

11. Model suitability audit

12. Before/after comparison to pre-hardening runs

13. Architecture boundary table
    Include pass/fail for:

* treequest_used false
* local text only
* prompt schema populated
* no null-array schema
* explicit array/type rules
* JSON mode requested or unsupported status recorded
* low temperature
* strict parsing preserved
* parseability-aware status
* no misleading all-zero EBP score
* no proof/promotion language
* faithfulness not_assessed
* no secret leakage

14. EBP/PTW self-audit

* needMap
* needInvariant
* needToyCheck
* needNullModel
* needObstruction
* needFaithfulnessReview
* promotion status

15. Final recommended next ticket

* If accepted, recommend either:

  * real_provider_smoke_passed_release_candidate
  * or stable baseline freeze.
* If repairs are needed, give the smallest repair ticket.

## Review standard

Be strict and concrete.

Do not accept real-provider readiness merely because tests pass.

Do not reject just because one free-tier provider fails.

Separate:

```text
provider failure
schema noncompliance
evaluator defect
physics-quality limitation
```

Do not claim the paper is true.

Do not claim the LLMs are physics authorities.

This review measures real-provider operational usability and schema-contract success only.
