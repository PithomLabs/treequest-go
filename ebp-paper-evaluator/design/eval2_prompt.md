You are a senior Go release reviewer, LLM-evaluator diagnostics engineer, and EBP/PTW audit planner.

Write an implementation plan for:

# EBP-EVAL-SIMPLE-0006: Real-Provider Degraded Smoke Acceptance and Agreement Diagnostics

## Background

`EBP-EVAL-SIMPLE` now has a `simple_triple_review` mode:

```text
local .txt/.md physics paper
+ EBP 2.1 policy/profile
+ same simple instruction to three independent reviewers:
  "Apply EBP 2.1 on the attached physics paper."
        ↓
three LLM responses
        ↓
strict JSON parsing
        ↓
nine evaluator-computed metrics
        ↓
agreement/disagreement ledgers
        ↓
candidate-scoped report
```

TreeQuest is not used in this mode.

Recent real-provider smoke results after schema-contract hardening:

```text
bundle: /home/chaschel/Desktop/physics/sakana/out/my-paper-triple-review

reviewer_1:
  model: google/gemma-4-31b-it:free
  status: reviewer_call_failed
  parse_status: not_applicable_no_content

reviewer_2:
  model: poolside/laguna-m.1:free
  parse_status: parsed
  score: 0.739

reviewer_3:
  model: nex-agi/nex-n2-pro:free
  parse_status: parsed
  score: 0.826

RunStatusSummary:
  call_run_status: partial_triple_review_two_reviewers
  parse_run_status: partial_reviews_parseable
  assessment_status: degraded_candidate_assessment_available
  returned_response_count: 2
  parseable_review_count: 2
  parseable_only_mean_reviewer_score: 0.782
  agreement_score: 0.000
  agreement_status: partial
```

This supersedes earlier failed smoke runs with zero parseable responses.

The latest result proves real-provider parseability for two responding models, but it does **not** prove full triple-review readiness or semantic convergence. The agreement score remained zero, meaning the two parseable reviewers did not overlap enough under current lexical/Jaccard claim matching.

## Goal

Accept the latest smoke result as a **degraded-but-usable real-provider smoke pass** while adding diagnostics to explain low agreement.

The goal is not to improve physics truth evaluation.

The goal is to make the real-provider status honest, inspectable, and useful:

```text
real_provider_status = degraded_but_usable
because:
  2 of 3 reviewers returned parseable strict JSON
  1 provider failed
  agreement diagnostics show why lexical agreement was low
```

## Non-negotiable boundaries

Do not:

* modify `treequest-go`;
* use TreeQuest in `simple_triple_review`;
* add embeddings or semantic search in this ticket;
* claim semantic convergence was achieved;
* claim the paper is true;
* claim EBP promotion;
* claim human faithfulness review;
* claim source-faithful TreeQuest parity;
* treat reviewer agreement as truth.

Preserve:

* local `.txt` / `.md` only;
* prompt trust boundary;
* strict JSON parsing;
* evaluator-computed scores;
* candidate-only report language;
* no-faithfulness profile behavior;
* artifact safety;
* `treequest_used: false`.

## Required implementation areas

### 1. Add real-provider smoke acceptance status

Add a clear status classification in the run/report layer:

```text
real_provider_status:
  full_triple_review_ready
  degraded_but_usable
  diagnostic_only
  not_usable
```

Classification rules:

```text
full_triple_review_ready:
  returned_response_count == 3
  parseable_review_count == 3
  assessment_status == candidate_assessment_available

degraded_but_usable:
  returned_response_count >= 2
  parseable_review_count >= 2
  assessment_status == degraded_candidate_assessment_available
  no secret leakage
  no proof/promotion language

diagnostic_only:
  returned_response_count > 0
  parseable_review_count == 0
  assessment_status == assessment_unavailable_schema_parse_failed

not_usable:
  returned_response_count == 0
  or artifact bundle is incomplete
  or secret/proof/promotion boundary fails
```

For the latest run, expected classification:

```text
real_provider_status = degraded_but_usable
```

### 2. Add model suitability ledger

Create or extend an artifact:

```text
consensus/model_suitability.json
```

Suggested schema:

```json
{
  "schema_version": "model-suitability-v0.1",
  "mode": "simple_triple_review",
  "reviewers": [
    {
      "reviewer_id": "reviewer_1",
      "model_id": "google/gemma-4-31b-it:free",
      "call_status": "reviewer_call_failed",
      "parse_status": "not_applicable_no_content",
      "score": 0.0,
      "suitability": "provider_unreliable",
      "notes": "Provider/API call failed before returning usable content."
    },
    {
      "reviewer_id": "reviewer_2",
      "model_id": "poolside/laguna-m.1:free",
      "call_status": "response_returned",
      "parse_status": "parsed",
      "score": 0.739,
      "suitability": "schema_compliant_in_latest_run",
      "notes": "Strict JSON parse succeeded."
    },
    {
      "reviewer_id": "reviewer_3",
      "model_id": "nex-agi/nex-n2-pro:free",
      "call_status": "response_returned",
      "parse_status": "parsed",
      "score": 0.826,
      "suitability": "schema_compliant_in_latest_run",
      "notes": "Strict JSON parse succeeded."
    }
  ]
}
```

Allowed suitability values:

```text
schema_compliant_in_latest_run
usable_with_caution
not_schema_compliant
provider_unreliable
unknown
```

This is not a physics-quality rating. It is only an operational/schema-compliance rating for this evaluator mode.

### 3. Add agreement diagnostics for low lexical overlap

The current agreement score can be zero even when two reviews parse successfully. Add diagnostics explaining why.

Create or extend:

```text
consensus/agreement_diagnostics.json
```

Include:

```json
{
  "schema_version": "agreement-diagnostics-v0.1",
  "agreement_status": "partial",
  "agreement_score": 0.0,
  "parseable_review_count": 2,
  "matching_method": {
    "type": "lexical_jaccard",
    "normalization": [
      "case_fold",
      "punctuation_removal",
      "whitespace_collapse",
      "stable_tokenization"
    ],
    "threshold": 0.60
  },
  "explanation": "Agreement is lexical/semantic-lite only. A low score may mean reviewers focused on different claims or expressed similar claims with insufficient lexical overlap.",
  "pairwise": [
    {
      "reviewer_a": "reviewer_2",
      "reviewer_b": "reviewer_3",
      "score": 0.0,
      "closest_claim_pairs": [
        {
          "claim_a_id": "claim_1",
          "claim_b_id": "claim_3",
          "claim_a_normalized": "...",
          "claim_b_normalized": "...",
          "jaccard": 0.42,
          "below_threshold": true
        }
      ]
    }
  ],
  "diagnostic_categories": [
    "different_focus",
    "paraphrase_not_captured_by_lexical_matching",
    "weak_grounding",
    "possible_overmerge_or_undermerge"
  ]
}
```

Do not add embeddings or model-based semantic similarity in this ticket. Only expose better lexical diagnostics.

### 4. Add closest-pair claim diagnostics

For each pair of parseable reviewers:

* normalize every claim text;
* compute token Jaccard for all cross-reviewer claim pairs;
* record top N closest pairs, even if below threshold;
* record which claims were unique because they failed to cross threshold;
* show threshold and score.

Suggested function:

```go
func BuildAgreementDiagnostics(results []ReviewerResult, ledger AgreementLedger) AgreementDiagnostics
```

Add tests for:

```text
TestAgreementDiagnostics_RecordsClosestPairsBelowThreshold
TestAgreementDiagnostics_ExplainsZeroAgreementWithTwoParseableReviews
TestAgreementDiagnostics_DoesNotClaimSemanticConvergence
```

### 5. Improve report language for degraded real-provider runs

Update `triple_review_report.md` generation.

For `real_provider_status = degraded_but_usable`, include:

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

Always retain required limitations:

```text
This is an automated candidate EBP assessment.
It is not a proof of the paper's claims.
It is not full EBP promotion.
Human faithfulness review was not performed.
The three LLM reviews are comparison signals, not authorities.
```

### 6. Preserve parseability and prompt-observability fields

Ensure artifacts still expose:

```text
call_run_status
parse_run_status
assessment_status
returned_response_count
parseable_review_count
temperature
response_format or provider_response_format_unsupported
document_hash
policy_hash
profile_id
user_message_hash
system_prompt_hash
schema_example_hash
```

If not currently implemented, include this as part of the plan.

### 7. Add regression fixture for latest degraded smoke run

Create a small test fixture or synthetic fixture representing:

```text
reviewer_1 = provider call failed
reviewer_2 = parsed score 0.739
reviewer_3 = parsed score 0.826
agreement_score = 0.000
parseable_review_count = 2
assessment_status = degraded_candidate_assessment_available
```

Add tests:

```text
TestRealProviderStatus_DegradedButUsable
TestModelSuitabilityLedger_TwoParsedOneProviderFailed
TestReport_DegradedButUsableLanguage
TestAgreementDiagnostics_ZeroAgreementPartialRun
```

### 8. Optional model recommendation output

Add a small report section:

```text
Model operational notes:
- poolside/laguna-m.1:free parsed successfully in latest smoke run.
- nex-agi/nex-n2-pro:free parsed successfully in latest smoke run.
- google/gemma-4-31b-it:free failed provider/API call in latest smoke run.
```

Do not claim these models know physics.

Only classify their operational suitability for this schema contract.

## Required files likely affected

Suggest concrete files in `ebp-paper-evaluator` only:

```text
pkg/simple/types.go
pkg/simple/agreement.go
pkg/simple/scoring.go
pkg/simple/artifacts.go
pkg/simple/triple_review.go
pkg/simple/*_test.go
cmd/ebp-paper-evaluator/main.go only if CLI/report fields need wiring
README.md
```

No `treequest-go` changes.

## Required tests

Add or update tests:

```text
TestRealProviderStatus_FullTripleReviewReady
TestRealProviderStatus_DegradedButUsable
TestRealProviderStatus_DiagnosticOnly
TestRealProviderStatus_NotUsable

TestModelSuitabilityLedger_TwoParsedOneProviderFailed
TestModelSuitabilityLedger_NotPhysicsAuthorityRating

TestAgreementDiagnostics_RecordsClosestPairsBelowThreshold
TestAgreementDiagnostics_ExplainsZeroAgreementWithTwoParseableReviews
TestAgreementDiagnostics_DoesNotClaimSemanticConvergence

TestReport_DegradedButUsableLanguage
TestReport_LowAgreementDoesNotClaimSemanticDisagreement
TestReport_KeepsRequiredLimitations

TestArtifacts_IncludeAgreementDiagnostics
TestArtifacts_IncludeModelSuitability
TestArtifacts_StillNoSecrets

TestPromptLedger_RecordsSchemaAndResponseFormatHashes
```

Existing tests must continue passing.

## Validation commands

Run:

```bash
go test ./...
go test -race ./...
go vet ./...
```

Run mock triple review:

```bash
ebp-paper-evaluator triple-review \
  --paper ./testdata/papers/local_physics_paper.txt \
  --policy ./policies/ebp_v2_1.md \
  --profile automated-no-faithfulness \
  --mock \
  --out /tmp/ebp-eval-simple-triple-review
```

Run non-EBP mock triple review:

```bash
ebp-paper-evaluator triple-review \
  --paper ./testdata/papers/local_physics_paper.txt \
  --policy ./testdata/policies/simple_review_policy.md \
  --mock \
  --out /tmp/ebp-eval-simple-triple-review-non-ebp
```

Optionally rerun real-provider smoke with the same models:

```bash
ebp-paper-evaluator triple-review \
  --paper ./testdata/papers/local_physics_paper.txt \
  --policy ./policies/ebp_v2_1.md \
  --profile automated-no-faithfulness \
  --models google/gemma-4-31b-it:free,poolside/laguna-m.1:free,nex-agi/nex-n2-pro:free \
  --out /tmp/ebp-eval-simple-triple-review-real-provider-smoke
```

Do not print or store API keys.

## Required implementation-plan output structure

Return the implementation plan in this exact structure:

1. Verdict and scope
2. Latest smoke-run interpretation
3. Files/functions likely affected
4. Real-provider status design
5. Model suitability ledger design
6. Agreement diagnostics design
7. Report wording changes
8. Artifact/provenance additions
9. Test plan
10. Validation plan
11. Risks and mitigations
12. Explicit non-goals
13. Acceptance criteria
14. EBP/PTW self-audit

## Acceptance criteria

This ticket is accepted when:

* latest degraded real-provider smoke pattern can be represented honestly;
* `real_provider_status = degraded_but_usable` is emitted for 2 parseable / 1 provider-failed runs;
* model suitability ledger is generated;
* agreement diagnostics explain zero agreement despite two parseable reviews;
* report clearly distinguishes:

  * parseability success,
  * provider failure,
  * low lexical agreement,
  * lack of semantic-convergence proof;
* no embeddings or semantic model are introduced;
* candidate-only limitation language remains;
* no faithfulness review is claimed;
* no EBP promotion is claimed;
* no secrets leak;
* TreeQuest remains unused;
* `go test ./...`, `go test -race ./...`, and `go vet ./...` pass;
* mock EBP and non-EBP runs still pass.

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

```text
This ticket accepts degraded real-provider smoke usability only.
It does not prove the paper.
It does not prove semantic convergence.
It does not validate physics.
It does not perform human faithfulness review.
It does not promote EBP claims.
It does not establish TreeQuest parity.
```

## Review standard

Keep this ticket small.

Do not solve semantic convergence yet.

Do not introduce embeddings.

Do not introduce PocketFlow.

Do not reintroduce TreeQuest.

Do not make provider failure look like reviewer disagreement.

Do not make zero lexical agreement look like semantic contradiction.

The correct philosophy is:

```text
Parseability is now usable for two responding models.
Agreement remains diagnostic, not authoritative.
The report must make both facts obvious.
```
