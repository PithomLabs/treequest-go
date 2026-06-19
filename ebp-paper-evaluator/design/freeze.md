# Implementation Plan - EBP-EVAL-SIMPLE-0007: simple_triple_review v0.1 Release Freeze

This plan freezes `simple_triple_review` as a stable v0.1 baseline. The goal is to verify, document, and lock the current working behavior without introducing new features, so that future TreeQuest refinement can be developed separately without destabilizing the simple baseline.

## User Review Required

> [!IMPORTANT]
> This release freeze locks the `simple_triple_review` v0.1 behavior. No new features, semantic embeddings, or TreeQuest integrations will be introduced in this ticket.
> 
> A new checklist file will be created at [simple_triple_review_v0_1_release_checklist.md](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/docs/releases/simple_triple_review_v0_1_release_checklist.md) to serve as the formal release checklist.

## Open Questions

None. The scope is well-defined and strictly concerns baseline stability, documentation, regression testing, and verification.

---

## Proposed Changes

### Configuration and Documentation

#### [NEW] [simple_triple_review_v0_1_release_checklist.md](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/docs/releases/simple_triple_review_v0_1_release_checklist.md)
* Create a release checklist file to audit compliance with v0.1 parameters.

#### [MODIFY] [README.md](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/README.md)
* Document `real_provider_status` logic and statuses.
* Document `model_suitability.json` and `agreement_diagnostics.json` structure, purpose, and semantics.
* Document temperature (`0.1`), `response_format` (`json_object`), prompt hashing, and the limitations of lexical matching (clarifying that a zero agreement score does not imply semantic disagreement).
* Highlight safety gates and no-promotion limits.

---

### Verification and Regression Testing

#### [MODIFY] [triple_review_test.go](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/simple/triple_review_test.go)
* Add 10 regression tests to verify that the simple triple review contract is locked:
  1. `TestReleaseV01_ArtifactLayout`: Verifies the presence of all generated folders and files inside `out/triple-review/`.
  2. `TestReleaseV01_ProvenanceContainsRequiredFields`: Verifies `run/provenance.json` contains `temperature`, `response_format`, `user_message_hash`, `schema_example_hash`, `system_prompt_hash`, and `real_provider_status`.
  3. `TestReleaseV01_TreeQuestUsedFalse`: Verifies that `treequest_used` is explicitly `false` in provenance.
  4. `TestReleaseV01_NoProofPromotionLanguage`: Checks that output reports and reviewers do not contain affirmative promotion/proof language.
  5. `TestReleaseV01_FaithfulnessNotAssessed`: Verifies faithfulness-humility disclosures report that human faithfulness review was not performed.
  6. `TestReleaseV01_ModelSuitabilityEmitted`: Verifies `consensus/model_suitability.json` exists and correctly populates suitability/error category fields.
  7. `TestReleaseV01_AgreementDiagnosticsEmitted`: Verifies `consensus/agreement_diagnostics.json` logs Jaccard token similarities and includes disclaimers.
  8. `TestReleaseV01_LocalTextOnly`: Asserts that non-local or non-text (`.pdf`) inputs are rejected by the ingestor.
  9. `TestReleaseV01_MockEBPBundleComplete`: Asserts a complete bundle is successfully produced with mock inputs under the EBP policy.
  10. `TestReleaseV01_MockNonEBPBundleComplete`: Asserts a complete bundle is successfully produced using the mock client and the `simple_review_policy.md` custom non-EBP policy.

---

## Release Capability Statement

`simple_triple_review v0.1` is a CLI-only local-text candidate review tool.
It can run three independent reviewer models, parse strict JSON responses, compute evaluator-owned quality metrics, report model operational suitability, and produce lexical agreement diagnostics.
It is not a truth engine, proof engine, semantic-convergence engine, or human faithfulness-review substitute.

---

## Stable Artifact Contract (v0.1 Directory Layout)

```text
out/triple-review/
  source/
    source_ref.json
    source_hash.txt
    original.txt                 # only with --copy-source
  policy/
    policy_snapshot.md
    policy_ir.json
    policy_hash.txt
  reviews/
    reviewer_1_raw.txt
    reviewer_1_parsed.json
    reviewer_1_score.json
    reviewer_1_error.json, if failed
    reviewer_2_raw.txt
    ...
    reviewer_3_raw.txt
    ...
  consensus/
    agreement_ledger.json
    disagreement_ledger.json
    combined_claims.json
    scoring_summary.json
    model_suitability.json
    agreement_diagnostics.json
  report/
    triple_review_report.md
  run/
    provenance.json
    budget_usage.json
    prompt_ledger.json
```

---

## Real-Provider Smoke Acceptance Detail

The latest live smoke run performed at `/home/chaschel/Desktop/physics/sakana/out/my-paper-triple-review` serves as our release verification baseline:
* **Status:** `real_provider_status: full_triple_review_ready`
* **Models:** `poolside/laguna-xs.2:free`, `poolside/laguna-m.1:free`, and `nex-agi/nex-n2-pro:free` returned parseable strict JSON.
* **Agreement:** `0.000` (Jaccard lexical token overlap below 0.60 threshold). Closest claim pairs were correctly identified and logged in `agreement_diagnostics.json` without claiming semantic convergence.
* **Parameters:** `temperature: 0.1`, `response_format: "json_object"`, `treequest_used: false`.

---

## Future TreeQuest v0.2 Note

A future-work stub will be added under ticket **EBP-EVAL-TQ-0001: TreeQuest-Guided Review Refinement** (v0.2+):
* **Input:** Existing `simple_triple_review` v0.1 bundle.
* **Goal:** Budget-aware claim, grounding, null models, and obstruction refinement.
* **TreeQuest role:** Search assistant, not authority.
* *Note:* TreeQuest is completely separate and is not part of `simple_triple_review` v0.1.

---

## EBP/PTW Self-Audit & Limits

* **needMap / needInvariant / needToyCheck / needNullModel / needObstruction:** Evaluated metrics are candidate-scoped only.
* **needFaithfulnessReview:** Marked `not_assessed`.
* **Promotion Status:** `simple_triple_review_v0_1_frozen` (after validation checks pass).
* **Affirmation:** This ticket freezes a candidate-review software baseline. It does not prove any paper, validate physics, prove semantic convergence, perform human faithfulness review, promote EBP claims, or establish TreeQuest parity.

---

## Verification Plan

### Automated Tests
* Run the full suite of unit tests, race detector, and linter checks:
  ```bash
  go test ./...
  go test -race ./...
  go vet ./...
  ```
* Run the mock EBP command:
  ```bash
  go run ./cmd/ebp-paper-evaluator triple-review \
    --paper ./testdata/papers/local_physics_paper.txt \
    --policy ./policies/ebp_v2_1.md \
    --profile automated-no-faithfulness \
    --mock \
    --out /tmp/ebp-eval-simple-v0-1-mock-ebp
  ```
* Run the mock non-EBP command:
  ```bash
  go run ./cmd/ebp-paper-evaluator triple-review \
    --paper ./testdata/papers/local_physics_paper.txt \
    --policy ./testdata/policies/simple_review_policy.md \
    --mock \
    --out /tmp/ebp-eval-simple-v0-1-mock-non-ebp
  ```
* Verify files are written to the output directories matching the v0.1 artifact contract layout.
