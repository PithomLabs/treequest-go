# Walkthrough - EBP-EVAL-SIMPLE-0007: simple_triple_review v0.1 Release Freeze

We have successfully frozen `simple_triple_review` as a stable v0.1 baseline, including full verification, documentation, checklist auditing, manifest recording, and regression tests.

## Changes Made

### 1. Release Boundary and Checklists
* Created the release checklist file at [simple_triple_review_v0_1_release_checklist.md](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/docs/releases/simple_triple_review_v0_1_release_checklist.md) auditing compliance with v0.1 boundaries.
* Created the machine-readable release manifest at [simple_triple_review_v0_1_manifest.json](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/docs/releases/simple_triple_review_v0_1_manifest.json) recording the locked release metadata:
  ```json
  {
    "release_id": "simple_triple_review_v0_1",
    "release_status": "simple_triple_review_v0_1_frozen",
    "mode": "simple_triple_review",
    "treequest_used": false,
    "local_text_only": true,
    "strict_json_parser": true,
    "real_provider_status_baseline": "full_triple_review_ready",
    "faithfulness_status": "not_assessed",
    "semantic_convergence_claimed": false,
    "ebp_promotion_claimed": false
  }
  ```

### 2. README Documentation Updates
* Modified [README.md](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/README.md) to add:
  * Detailed description of the `real_provider_status` logic and statuses (`full_triple_review_ready`, `degraded_but_usable`, `diagnostic_only`, `not_usable`).
  * Explanations of `model_suitability.json` and its failure categories.
  * Explanations of `agreement_diagnostics.json` closest-pair logs and lexical Jaccard limitations.
  * Documented provenance fields (temperature `0.1`, `json_object` format, prompt/schema hashes).
  * Updated artifact layout list to include all 6 consensus json artifacts.

### 3. Regression Test Hardening
* Implemented 10 regression tests in [triple_review_test.go](file:///home/chaschel/Documents/go/treequest/ebp-paper-evaluator/pkg/simple/triple_review_test.go):
  1. `TestReleaseV01_ArtifactLayout`: Verifies the presence of all 6 subfolders and 24 files inside the output bundle.
  2. `TestReleaseV01_ProvenanceContainsRequiredFields`: Asserts provenance matches the contract fields.
  3. `TestReleaseV01_TreeQuestUsedFalse`: Asserts `treequest_used` is explicitly `false`.
  4. `TestReleaseV01_NoProofPromotionLanguage`: Verifies report and reviewer prompts contain no affirmative promotional language.
  5. `TestReleaseV01_FaithfulnessNotAssessed`: Asserts that faithfulness limits are correctly disclosed as unassessed.
  6. `TestReleaseV01_ModelSuitabilityEmitted`: Verifies `consensus/model_suitability.json` exists and parses correctly.
  7. `TestReleaseV01_AgreementDiagnosticsEmitted`: Verifies `consensus/agreement_diagnostics.json` logs similarity statistics.
  8. `TestReleaseV01_LocalTextOnly`: Asserts that PDF ingestion is rejected by the local text ingestor.
  9. `TestReleaseV01_MockEBPBundleComplete`: Verifies a complete EBP evaluation bundle is produced with mock inputs.
  10. `TestReleaseV01_MockNonEBPBundleComplete`: Verifies a complete bundle is generated using a custom non-EBP policy.

---

## Verification and Validation Results

All automated validation tests passed successfully:

1. **Unit tests (`go test ./...`):** **PASS**
2. **Race detector checks (`go test -race ./...`):** **PASS**
3. **Linter checks (`go vet ./...`):** **PASS**

### Manual Mock Runs
* **EBP Mock Run:**
  ```bash
  go run ./cmd/ebp-paper-evaluator triple-review --paper ./testdata/papers/local_physics_paper.txt --policy ./policies/ebp_v2_1.md --profile automated-no-faithfulness --mock --out /tmp/ebp-eval-simple-v0-1-mock-ebp
  ```
  Result: `/tmp/ebp-eval-simple-v0-1-mock-ebp` written with 6 correct subdirectories and 24 files.
* **Non-EBP Mock Run:**
  ```bash
  go run ./cmd/ebp-paper-evaluator triple-review --paper ./testdata/papers/local_physics_paper.txt --policy ./testdata/policies/simple_review_policy.md --mock --out /tmp/ebp-eval-simple-v0-1-mock-non-ebp
  ```
  Result: `/tmp/ebp-eval-simple-v0-1-mock-non-ebp` generated successfully.


- `[x]` Create `docs/releases/simple_triple_review_v0_1_release_checklist.md`
- `[x]` Create `docs/releases/simple_triple_review_v0_1_manifest.json`
- `[x]` Update `README.md` with documentation on diagnostics, suitability, status, hashes, and limitations
- `[x]` Add regression tests in `pkg/simple/triple_review_test.go`
- `[x]` Run validation checks (`go test ./...`, `go test -race ./...`, and `go vet ./...`)
- `[x]` Run mock verification commands and assert bundle completeness
