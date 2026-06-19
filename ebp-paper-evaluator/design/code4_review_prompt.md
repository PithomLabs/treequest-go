You are an adversarial senior Go reviewer, LLM-safety auditor, and EBP/PTW artifact auditor.

Review the completed implementation:

# EBP-EVAL-SIMPLE-0002: Simple Triple Review Post-Implementation Audit

## Reported implementation

The user reports that `EBP-EVAL-SIMPLE-0001` has been implemented in `ebp-paper-evaluator`.

Reported status:

```text
EBP-EVAL-SIMPLE-0001 completed.
simple_triple_review_mode_ready.
treequest_used: false.
CANDIDATE_REPORT_COMPLETE_HUMAN_REVIEW_REQUIRED.
```

Reported implementation details:

* Added `pkg/simple`.
* Added `triple-review` CLI mode.
* Added tests, README updates, and a local physics fixture.
* TreeQuest was not used or modified.
* `treequest-go` was not modified.
* Prompt design uses trusted system policy/schema plus identical user instruction and escaped untrusted paper.
* Reviewer outputs use strict JSON with required-field and unknown-field validation.
* Scoring implements all nine required metrics.
* Review quality is separated from run completeness.
* Agreement artifacts include shared, partially shared, unique, conflicting, debt, obstruction, and grounding ledgers.
* Bundles were generated at:

```text
/tmp/ebp-eval-simple-triple-review
/tmp/ebp-eval-simple-triple-review-non-ebp
```

Reported validation:

```bash
go test ./...        PASS
go test -race ./...  PASS
go vet ./...         PASS
EBP mock run         PASS
non-EBP mock run     PASS
```

Known limitations:

* Lexical matching only.
* Relative claim coverage does not measure scientific truth.
* Faithfulness remains `not_assessed`.

## Review objective

Determine whether `simple_triple_review` should be accepted for the current mock/e2e v0.1 scope, or whether repairs are required before treating it as a stable simple-review baseline.

Return one verdict:

```text
accept
accept_with_repairs
reject_for_now
```

## Non-negotiable boundaries

The implementation must satisfy all of these:

1. `treequest-go` is unchanged.
2. `pkg/simple` imports or invokes no TreeQuest APIs.
3. `simple_triple_review` provenance records:

```json
{
  "mode": "simple_triple_review",
  "treequest_used": false
}
```

4. The evaluator remains CLI-only and local `.txt` / `.md` only.
5. URL, arXiv, PDF, HTTP, browser, OCR, and `pdftotext` paper ingestion remain absent.
6. The same exact user message is sent to all three reviewers.
7. Reviewer identity is not embedded in user content.
8. Reviewer identity, task, policy/profile, schema, and safety rules live in system/metadata.
9. Paper text is escaped and wrapped as untrusted evidence.
10. Policy content is trusted configuration, not mixed into paper evidence.
11. Three reviewer calls run concurrently but artifacts are sorted deterministically.
12. One reviewer failure does not abort the run if at least one reviewer returns content.
13. Zero reviewer content produces `run_failed_no_reviewer_content` and no completed assessment bundle.
14. Malformed reviewer JSON is preserved as raw with `review_parse_failed`.
15. Provider failure is preserved as sanitized `reviewer_call_failed`.
16. Scores are computed by evaluator code, not accepted from LLMs.
17. Final score is equal-weight mean of the nine evaluator-computed metrics.
18. Review quality and run completeness are separate fields.
19. Agreement is lexical/semantic-lite only, not claimed as full semantic understanding.
20. Faithfulness remains `not_assessed`.
21. Reports contain candidate-only limitation language.
22. No report claims proof, scientific truth, EBP promotion, human faithfulness review, or source-faithful TreeQuest parity.
23. No secrets or API keys appear in artifacts.

## Required audit areas

### A. Repository and TreeQuest boundary audit

Run or inspect:

```bash
git status --short
git diff --stat
git diff -- treequest-go
grep -R "treequest" -n ebp-paper-evaluator/pkg/simple ebp-paper-evaluator/cmd/ebp-paper-evaluator
```

Verify:

* `treequest-go` unchanged.
* `pkg/simple` has no TreeQuest imports.
* CLI does not route `triple-review` through TreeQuest.
* Bundle/provenance says `treequest_used: false`.
* No TreeQuest snapshots are produced for simple mode.

Classify any TreeQuest reference as:

```text
allowed documentation/provenance only
needs explanation
release blocker
```

### B. CLI audit

Review `triple-review` CLI.

Check:

* command exists and is documented
* `--paper` is required
* `--input-root` is respected
* `--max-paper-bytes` is respected
* `--policy` is required or has documented safe default
* `--profile` works
* `--allow-untrusted-policy` behavior is explicit
* `--provider` behavior is documented
* `--models` requires exactly three distinct non-empty model IDs in non-mock mode
* `--mock` uses deterministic mock reviewers
* `--timeout` applies to all reviewer calls
* `--out` is required or defaulted safely
* `--include-local-paths` and `--copy-source` preserve privacy by default
* remote paper inputs are rejected
* invalid model list errors are clear
* provider creation is not attempted when input validation fails

### C. Local-ingestion regression audit

Confirm simple mode reuses existing safe local ingestion:

* local `.txt` / `.md` only
* rejects URL strings
* rejects unsupported extensions
* rejects directories
* rejects symlinks or root escapes
* rejects non-UTF-8
* enforces size limit
* hashes original bytes
* redacts paths by default

Ensure simple mode did not bypass the hardened local-ingestion layer.

### D. Prompt boundary audit

Inspect prompt builders and prompt ledger.

Verify:

* every reviewer receives identical user content
* user content begins with the exact simple instruction:

```text
Apply EBP 2.1 on the attached physics paper.
```

* the same escaped `<untrusted_paper>` payload follows
* paper content cannot escape the wrapper
* reviewer identity is excluded from user message
* reviewer identity appears only in system/metadata
* policy/schema/safety rules appear only in trusted system or metadata
* hostile paper text is treated as source text only
* prompt hashes are stable
* prompt ledger records role/task/model/prompt hash

Required tests or equivalent evidence:

```text
TestTripleReview_SimpleUserInstructionPreserved
TestTripleReview_ThreeReviewersReceiveSameUserMessage
TestTripleReview_ReviewerIdentityNotInUserMessage
TestTripleReview_PaperWrappedAsUntrusted
TestTripleReview_HostilePaperDoesNotOverrideSystem
```

### E. Concurrent execution and deterministic artifacts audit

Verify:

* one goroutine per reviewer or equivalent concurrency
* shared timeout
* per-reviewer isolation
* no data races under `go test -race`
* result collection does not depend on completion order
* prompt ledger sorted by reviewer ID
* review files sorted by reviewer ID
* scoring summary sorted deterministically
* fixed injected clock is used in determinism tests
* bundle digest is completion-order-independent

Required tests or equivalent evidence:

```text
TestTripleReview_ConcurrentCallsComplete
TestTripleReview_ArtifactsSortedByReviewerID
TestTripleReview_CompletionOrderDoesNotChangeBundleHash
TestTripleReview_ConcurrentBudgetAccountingPerReviewer
```

### F. Partial failure audit

Check all four run-status cases:

```text
3 responses: triple_review_complete
2 responses: partial_triple_review_two_reviewers
1 response: partial_review_insufficient_for_agreement
0 responses: run_failed_no_reviewer_content
```

Verify:

* failed reviewer produces `reviewer_N_error.json`
* malformed reviewer produces raw response and score file
* malformed reviewer does not poison other reviewers
* provider failure is sanitized
* at least one returned response can produce a degraded bundle
* zero returned responses does not emit completed assessment bundle
* report clearly marks partial/degraded runs

Required tests:

```text
TestTripleReview_ThreeCompletedReviewers
TestTripleReview_TwoCompletedOneFailed
TestTripleReview_OneCompletedTwoFailed
TestTripleReview_ZeroCompletedFailsNoBundle
TestTripleReview_MalformedReviewerJSONStoredAsRawFailure
```

### G. Strict JSON parsing audit

Review reviewer parsing.

Confirm:

* first balanced JSON object extraction is used
* unknown fields rejected
* required fields validated
* expected reviewer/model IDs validated
* candidate status validated
* limitation language validated
* malformed content preserved as raw
* LLM-provided scores are ignored or rejected
* parsed reviewer output cannot contain proof/promotion language without penalty/failure

Search for unsafe parsing:

```bash
grep -R "LastIndex.*}" -n ebp-paper-evaluator/pkg ebp-paper-evaluator/cmd
grep -R "Index.*{" -n ebp-paper-evaluator/pkg ebp-paper-evaluator/cmd
```

### H. Scoring audit

Review all nine evaluator-computed metrics:

```text
claim_coverage
source_grounding
ebp_debt_coverage
map_invariant_quality
toy_null_obstruction_awareness
faithfulness_humility
no_overclaim_discipline
cross_model_agreement
next_step_usefulness
```

Verify:

* each score is bounded `[0.0, 1.0]`
* final score is equal-weight mean
* malformed/failed reviewers score `0.0`
* failed reviewer score is distinguished from run completeness
* `claim_coverage` is labeled relative, not truth-level coverage
* `source_grounding` requires exact normalized-paper quote matches
* section hints alone do not count as full grounding
* `ebp_debt_coverage` is driven by policy debt items
* `faithfulness_humility` enforces no-human-faithfulness language
* `no_overclaim_discipline` penalizes forbidden affirmative claims
* `cross_model_agreement` is `0.0` when fewer than two parseable reviews exist
* no LLM self-score is trusted

### I. Agreement/disagreement audit

Verify deterministic lexical agreement logic:

* case-folding
* punctuation removal
* whitespace collapse
* stable tokenization
* exact normalized match
* token Jaccard threshold `>= 0.60`
* deterministic clustering
* shared claims
* partially shared claims
* unique claims
* shared/unique debts
* shared obstructions
* weakly grounded claims
* disagreement ledger
* conflicts only when overlapping claims have explicit opposing negation polarity

Confirm the report does not imply deep semantic convergence beyond lexical/semantic-lite matching.

### J. Policy genericity audit

Run/inspect EBP and non-EBP policy fixtures.

Verify:

* non-EBP policy works in triple-review mode
* non-EBP debts and dimensions are not forced into EBP names
* EBP-specific debt IDs are allowed only in EBP policy/profile/test data
* generic scoring does not hardcode EBP debts except where explicitly converting EBP policy data
* policy forbidden-language checks are applied
* no hardcoded fallback to EBP when custom policy fails

Search:

```bash
grep -R "needMap\|needInvariant\|needToyCheck\|needNullModel\|needObstruction\|needFaithfulnessReview" -n ebp-paper-evaluator/pkg/simple ebp-paper-evaluator/pkg/eval ebp-paper-evaluator/pkg/policy
```

Classify hits.

### K. Artifact bundle audit

Inspect both bundles:

```text
/tmp/ebp-eval-simple-triple-review
/tmp/ebp-eval-simple-triple-review-non-ebp
```

Expected layout:

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
reviews/reviewer_1_raw.txt
reviews/reviewer_1_parsed.json
reviews/reviewer_1_score.json
reviews/reviewer_2_raw.txt
reviews/reviewer_2_parsed.json
reviews/reviewer_2_score.json
reviews/reviewer_3_raw.txt
reviews/reviewer_3_parsed.json
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

For malformed/failure fixtures, expected alternate files:

```text
reviews/reviewer_N_error.json
```

Check:

* all expected files exist
* hashes cross-reference correctly
* prompt ledger records role/task
* provenance records `treequest_used:false`
* model mapping exists
* run/comparison status exists
* budget usage is per reviewer
* source paths are redacted by default
* copied source hash matches if `--copy-source`
* no TreeQuest snapshots
* no old per-claim TreeQuest tree files in simple mode
* report includes limitation language

### L. Report audit

The final report must include:

```text
This is an automated candidate EBP assessment.
It is not a proof of the paper's claims.
It is not full EBP promotion.
Human faithfulness review was not performed.
The three LLM reviews are comparison signals, not authorities.
```

Verify the report includes:

* score table
* run completeness
* review quality distinction
* shared claims
* unique claims
* debts by reviewer
* agreement/disagreement summary
* weak grounding warnings
* recommended human review steps
* limitations

Reject if the report claims:

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

### M. Secret and privacy audit

Scan artifacts for:

```text
OPENROUTER_API_KEY
Authorization
Bearer
sk-
x-api-key
.env
provider request headers
absolute local paths, unless --include-local-paths was set
```

Verify credential-like content is rejected or redacted.

### N. README/docs audit

Confirm documentation states:

* simple triple-review is optional mode
* TreeQuest is not used in this mode
* local `.txt` / `.md` only
* no URL/PDF/arXiv ingestion
* three model IDs required in non-mock mode
* mock mode uses deterministic reviewers
* metrics are evaluator-computed
* lexical agreement is limited
* relative claim coverage is not paper truth coverage
* no faithfulness review
* no EBP promotion
* no scientific proof
* output is candidate assessment only

## Required validation commands

From `ebp-paper-evaluator`:

```bash
go test ./...
go test -race ./...
go vet ./...
```

Run mock EBP triple review:

```bash
ebp-paper-evaluator triple-review \
  --paper ./testdata/papers/local_physics_paper.txt \
  --policy ./policies/ebp_v2_1.md \
  --profile automated-no-faithfulness \
  --mock \
  --out /tmp/ebp-eval-simple-triple-review-audit
```

Run mock non-EBP triple review:

```bash
ebp-paper-evaluator triple-review \
  --paper ./testdata/papers/local_physics_paper.txt \
  --policy ./testdata/policies/simple_review_policy.md \
  --mock \
  --out /tmp/ebp-eval-simple-triple-review-non-ebp-audit
```

## Required output format

Return your review in this exact structure:

1. Verdict

   * `accept`, `accept_with_repairs`, or `reject_for_now`
   * one paragraph rationale

2. Critical blockers

   * must-fix before acceptance

3. High-priority repairs

   * important gaps

4. Medium-priority improvements

   * maintainability/docs/future hardening

5. TreeQuest boundary audit

6. Local-ingestion and CLI audit

7. Prompt-boundary audit

8. Concurrency and partial-failure audit

9. Parsing and scoring audit

10. Agreement/disagreement audit

11. Policy genericity audit

12. Artifact and report audit

13. Secret/privacy audit

14. Test gaps

15. Architecture boundary table
    Include pass/fail for:

* treequest-go unchanged
* `pkg/simple` no TreeQuest import
* `treequest_used:false`
* local `.txt` / `.md` only
* no URL/PDF/arXiv/HTTP/pdftotext
* identical user message
* reviewer identity excluded from user content
* untrusted paper wrapper
* concurrent reviewer calls
* deterministic artifact ordering
* 3/2/1/0 partial status handling
* strict JSON parsing
* evaluator-computed scores
* review quality separated from run completeness
* equal-weight final score
* lexical agreement limitation
* non-EBP policy fixture
* no-faithfulness not_assessed
* no proof/promotion language
* no secret leakage

16. EBP/PTW self-audit

* needMap
* needInvariant
* needToyCheck
* needNullModel
* needObstruction
* needFaithfulnessReview
* promotion status

17. Final recommended next ticket

* If accepted, recommend release freeze or real-provider smoke test.
* If repairs are needed, give the smallest repair ticket.

## Review standard

Be strict.

Do not accept unless simple triple-review is genuinely independent of TreeQuest, local-only, deterministic, prompt-safe, artifact-safe, and honest about limitations.

Do not require embeddings, semantic search, TreeQuest, PocketFlow, URL ingestion, PDF ingestion, or human faithfulness review.

Do not allow claims of paper truth, proof, EBP promotion, human faithfulness review, or TreeQuest parity.
