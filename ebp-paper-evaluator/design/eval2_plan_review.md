## Verdict

```text
accept_with_minor_repairs
```

This is a good plan. It correctly accepts the latest real-provider smoke run as **degraded-but-usable**, not fully ready. It also avoids the dangerous mistake of treating low lexical agreement as semantic disagreement. The plan keeps the ticket small: status classification, model suitability, agreement diagnostics, report wording, and provenance improvements. That is the right scope.

## What the plan gets right

The strongest parts are:

```text
PASS: no TreeQuest changes
PASS: no embeddings or semantic matching added
PASS: degraded_but_usable status defined clearly
PASS: model suitability is operational, not physics-authority ranking
PASS: agreement_score = 0 is explained as lexical/Jaccard limitation
PASS: report language remains candidate-scoped
PASS: prompt hashes, temperature, response_format added to provenance
PASS: agreement diagnostics expose closest claim pairs below threshold
```

The key improvement is that the report will now say:

```text
Two models parsed successfully.
One provider failed.
Agreement was low under lexical matching.
This does not prove semantic disagreement.
```

That is exactly the epistemic hygiene needed.

## Minor repairs before implementation

### 1. RealProviderStatus should include safety gates

Your current rule says:

```text
degraded_but_usable:
  returned >= 2
  parseable >= 2
```

Add safety gates:

```text
degraded_but_usable only if:
  parseable_review_count >= 2
  no secret leakage
  no proof/promotion/final-truth language
  artifact bundle complete
  treequest_used == false
```

Otherwise a technically parseable but unsafe artifact could be mislabeled usable.

### 2. Add `real_provider_status` to both provenance and scoring summary

Do not store it only in `Provenance`.

Add it to:

```text
run/provenance.json
consensus/scoring_summary.json
report/triple_review_report.md
```

Reason: users will inspect the scoring summary first.

### 3. Include provider failure category in model suitability

For reviewer 1, avoid only saying `provider_unreliable`. Add a machine-readable failure category:

```json
{
  "suitability": "provider_unreliable",
  "failure_category": "reviewer_call_failed",
  "error_summary": "sanitized provider/API failure"
}
```

This helps distinguish timeout, unsupported response format, rate limit, and provider outage later.

### 4. Agreement diagnostics should include top pairs even when agreement is zero

You already planned this. Make it explicit in acceptance:

```text
If agreement_score == 0 and parseable_review_count >= 2,
agreement_diagnostics.json must still contain closest_claim_pairs.
```

Otherwise the diagnostic artifact will not explain the zero score.

### 5. Preserve “not semantic convergence” in machine-readable form

Add a field:

```json
{
  "semantic_convergence_claimed": false,
  "agreement_method_limit": "lexical_jaccard_only"
}
```

This protects against future report readers overinterpreting agreement scores.

## Recommended status language

Use:

```text
EBP-EVAL-SIMPLE-0006 plan accepted with minor repairs.

The latest real-provider smoke run should be classified as degraded_but_usable: two real-provider reviewers returned strict-parseable JSON and one provider call failed.

The evaluator remains candidate-scoped. Agreement remains lexical-only and diagnostic. No semantic convergence, physics truth, human faithfulness review, EBP promotion, or TreeQuest parity is claimed.
```

## Best implementation order

```text
1. Add real_provider_status classification.
2. Add model_suitability.json.
3. Add agreement_diagnostics.json with closest-pair Jaccard details.
4. Add provenance/scoring_summary fields for prompt hashes, response_format, temperature.
5. Update report wording.
6. Add degraded smoke fixture tests.
7. Run go test ./..., go test -race ./..., go vet ./...
8. Re-run mock EBP and non-EBP triple review.
9. Optionally re-run real-provider smoke.
```

## EBP/PTW self-audit

**needMap:** Satisfied. The plan maps real-provider smoke results into status, suitability, diagnostics, and report artifacts.

**needInvariant:** Mostly satisfied. Add safety gates so `degraded_but_usable` cannot bypass secret/proof/promotion checks.

**needToyCheck:** Satisfied by the synthetic 2-parsed/1-provider-failed fixture.

**needNullModel:** The previous zero-parseable run remains the comparison baseline.

**needObstruction:** Remaining obstruction is low lexical agreement and provider reliability, not parser failure.

**needFaithfulnessReview:** Correctly remains `not_assessed`.

**Promotion status:** `implementation_plan_accepted_with_minor_repairs`; no physics validation, semantic convergence, EBP promotion, human faithfulness review, or TreeQuest parity claim.
