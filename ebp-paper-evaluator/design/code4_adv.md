# EBP-EVAL-SIMPLE-0002 Triple Review Post-Implementation Audit

## 1. Verdict

**accept_with_repairs**

The implementation is technically sound: `pkg/simple` is genuinely independent of TreeQuest, all local-ingestion guards are preserved, the three reviewer calls run concurrently with deterministic ordering, prompt boundaries hold (paper escaped inside `<untrusted_paper>`, reviewer identity excluded from user content, policy in system only), strict JSON parsing rejects unknowns and ignores LLM self-scores, all nine evaluator metrics are bounded and mean-reduced, partial-failure states (3/2/1/0) are handled correctly, and both EBP and non-EBP bundles regenerate cleanly with no leaked secrets. However, `pkg/eval/roles.go` continues to import `treequest-go/pkg/algo` and `treequest-go/pkg/tree` in the same binary as `triple-review`; while the `simple` package itself is boundary-clean, that import is visible to any tool scanning the running binary and violates the spirit of boundary 1. Additionally, one test redacts reviewer identity rather than explicitly asserting the absence of reviewer-ID strings in user prompts, leaving a small observable gap. These are repair-class issues, not release-blocking.

## 2. Critical blockers

None.

## 3. High-priority repairs

1. **Harden treequest binary boundary**: `pkg/eval/roles.go` (lines 12–13) imports `treequest-go/pkg/algo` and `treequest-go/pkg/tree`. These imports are reachable because `cmd/ebp-paper-evaluator/main.go` imports `ebp-paper-evaluator/pkg/eval`. The `triple-review` subcommand never calls `eval` code, but the Go linker pulls `eval` (and therefore `treequest-go`) into the `triple-review` binary. Build a `run`-only `eval` bridge, put `treequest`-using code behind a build-tag gate, or move `eval` into an internal sub-package not compiled for `triple-review`.

2. **Strengthen reviewer-identity-negative test**: `TestTripleReview_SimpleUserInstructionPreserved` (`pkg/simple/triple_review_test.go:161`) checks `!strings.Contains(r.User, r.Role)` — but `r.Role` is the prompt *sender's* role (the `simple` package itself), not the literal `reviewer_1`/`reviewer_2`/`reviewer_3` string. Add an explicit sub-test enumerating expected absent substrings (`"reviewer_1 through reviewer_3"`) in every reviewer's user prompt, matching the literal reviewer ID assigned by `reviewOne`.

## 4. Medium-priority improvements

1. **Add explicit partial-failure CLI integration tests** for the 2-of-3 and 1-of-3 cases in `cmd/ebp-paper-evaluator/main_test.go`, verifying the emitted error files and `run_status` field in the produced `provenance.json`.

2. **Design doc cleanup**: numerous `*.md` files remain at the repo root (e.g., `plan.md`, `code2_review.md`, `treequest_go_port.md`) that are audit noise. They are not blockers but make the workspace harder to diff-audit.

3. **Cross-model agreement when `parseable < 2`**: The code in `scoring.go:76` correctly returns `0.0`, but `agreement.go:188` sets the agreement score to `0` when `len(ids) < 2` — this is correct, but no unit test explicitly exercises the divergent `CrossModelAgreement` vs `AgreementScore` when only one reviewer parses. Add one.

## 5. TreeQuest boundary audit

| Check | Result |
|---|---|
| `treequest-go` unchanged | PASS — `git diff -- treequest-go` produced zero output |
| `pkg/simple` imports | PASS — `simple/*.go` imports only stdlib + app packages (`budget`, `document`, `jsonutil`, `llm`, `policy`) |
| CLI triple-review route | PASS — `tripleReview()` calls `simple.Run()` directly; no TreeQuest API invoked |
| Provenance `treequest_used: false` | PASS — emitted into both live bundles and confirmed at `/tmp/ebp-eval-*/*/run/provenance.json` |
| No TreeQuest snapshots in simple mode | PASS — `saveArtifacts` writes only `source/`, `policy/`, `reviews/`, `consensus/`, `report/`, `run/` |
| `pkg/eval` visibility to triple-review binary | CONCERN — `main.go` imports `eval`, which imports `treequest-go/pkg/algo` and `treequest-go/pkg/tree` at `main.go:17–18`. The `triple-review` subcommand does not call `eval`, but the linker pulls these symbols into the built binary. Classified as `needs explanation / repair before stable baseline` |

## 6. Local-ingestion and CLI audit

| Check | Result |
|---|---|
| `.txt` / `.md` only | PASS — `pkg/document/localfs.go:42` |
| Rejects URLs | PASS — `ErrRemotePaperUnsupported` at `main.go` test `TestTripleReview_RejectsRemotePaperInput` |
| Rejects unsupported extensions | PASS |
| Rejects directories | PASS (`info.IsDir()` check) |
| Rejects symlinks | PASS (`EvalSymlinks` + `ModeSymlink` double-check) |
| Rejects non-UTF-8 | PASS (`utf8.Valid`) |
| Enforces size limit | PASS — respects `--max-paper-bytes`, defaults to `DefaultMaxPaperBytes` |
| Hashes original bytes | PASS (`hashBytes(raw)` in `buildDocument`) |
| Redacts paths by default | PASS — `input_path: "./testdata/papers/..."` in source_ref.json; absolute path only when `--include-local-paths` |
| `--paper` required | PASS |
| `--models` exactly 3, distinct, non-empty | PASS |
| `--mock` deterministic | PASS |
| Shared timeout | PASS (`context.WithTimeout` wraps entire run) |
| Provider not instantiated when `--mock` | PASS — `if *mock { client = simpleMockClient(...) } else { client, err = llm.NewOpenRouterClient(...) }` |
| `--input-root` / `--copy-source` privacy default | PASS |
| `OPENROUTER_API_KEY` in source | INFORMATIONAL — `main.go:116,258` pass the key name as string literal to `NewOpenRouterClient`, which reads from env; no actual values in repo |

## 7. Prompt-boundary audit

| Check | Result |
|---|---|
| Identical user content to all 3 | PASS — `userPrompt(c)` is a single function call per `reviewOne`, requires match in `TestTripleReview_SimpleUserInstructionPreserved` |
| Exact prefix `Apply EBP 2.1 on the attached physics paper.` | PASS — `UserInstruction = ...` at `types.go:12`, verified by test prefix check |
| `<untrusted_paper>` wrapper | PASS — `html.EscapeString(c.Document.Hash)` + `html.EscapeString(paper.String())` embedded in tagged wrapper in `userPrompt` |
| Paper cannot escape wrapper | PASS — `TestTripleReview_PaperWrappedAsUntrusted` injects `</untrusted_paper>` in paper body; wrapper still intact |
| Reviewer identity excluded from user message | PASS (with caveat) — system prompt contains `Expected reviewer_id=%q`; user prompt contains only `UserInstruction` + paper. See High-priority #2: test needs strengthening |
| Reviewer identity in system/metadata only | PASS |
| Policy in system only | PASS — `TRUSTED_POLICY (configuration, not evidence)` prefix; enforced by `TestPrompt_PolicyNotMixedWithPaperEvidence` in `pkg/eval/prompt_test.go` |
| Hostile paper treated as source only | PASS — `TestTripleReview_HostilePaperDoesNotOverrideSystem` and `TestPromptInjection_HostilePaperDoesNotOverrideSystem` |
| Prompt hashes stable | PASS — `computePromptHash` is deterministic |

## 8. Concurrency and partial-failure audit

| Check | Result |
|---|---|
| One goroutine per reviewer | PASS — `go func(n int) { ... }(i)` loop |
| Shared timeout | PASS |
| No data races | PASS — `go test -race ./...` clean; channel is buffered to 3; `wg.Wait` done in separate goroutine |
| Result collection independent of completion order | PASS — results stored by `x.n` index before `summarize()` |
| Prompt ledger sorted by reviewer ID | PASS — `sortedPromptRecords` sorts by `Role` |
| Fixed injected clock in determinism test | PASS — `Clock: func() time.Time { return time.Unix(0,0).UTC() }` in `fixtureConfig` |
| Bundle hash completion-order-independent | PASS — `TestTripleReview_CompletionOrderDoesNotChangeBundleHash` passes |
| 3-of-3 → `triple_review_complete` | PASS |
| 2-of-3 → `partial_triple_review_two_reviewers` | PASS |
| 1-of-3 → `partial_review_insufficient_for_agreement` | PASS (`summarize()` explicitly sets it) |
| 0-of-3 → `run_failed_no_reviewer_content`, no bundle | PASS (`TestTripleReview_ZeroReturnsNoBundle`) |
| Failed reviewer → `reviewer_N_error.json` | PASS |
| Malformed reviewer → raw preserved + `review_parse_failed` | PASS |
| Failures do not poison others | PASS — each goroutine is fully isolated in error handling |

## 9. Parsing and scoring audit

| Check | Result |
|---|---|
| First balanced JSON object extraction | PASS — `jsonutil.ExtractFirstJSONObject` uses depth-aware scanner |
| Unknown fields rejected | PASS — `DecodeFirstJSONObject[ReviewerOutput]` with `dec.DisallowUnknownFields()` |
| Required fields validated | PASS — `validateOutput` |
| Malformed content preserved as raw | PASS |
| LLM-provided scores ignored | PASS — scores come from `scoreReviewer`, field `ReviewerOutput` has no numeric score |
| `claim_coverage` labeled relative | PASS — `ClaimCoverageMeaning` string set in scoring.go |
| `source_grounding` requires quote match | PASS — `claimGrounded` does exact normalized string search on paper text |
| `ebp_debt_coverage` driven by policy debt items | PASS — iterates `c.Policy.IR.DebtItems` |
| `faithfulness_humility` enforces no-human-faithfulness language | PASS — checks for exact phrase in reviewer `Limitations` |
| `no_overclaim_discipline` penalizes forbidden patterns | PASS — regex matches on `PaperSummary`, `OverallAssessment`, `RecommendedNextSteps` + policy forbidden patterns |
| `cross_model_agreement` = 0.0 when <2 parseable | PASS — `if len(sets) > 1` gate |
| Final score equal-weight mean of 9 | PASS — `vals` slice of 9, `total / 9` with `clamp` |
| Scores bounded [0.0, 1.0] | PASS — `clamp` enforces bounds |

## 10. Agreement/disagreement audit

| Check | Result |
|---|---|
| Case-folding, punctuation removal | PASS — `normalize` uses `[a-z0-9]+` regex |
| Jaccard threshold ≥ 0.60 | PASS — `bestScore < .60` |
| Deterministic clustering | PASS — clusters sorted by `normalize(CanonicalText)` |
| Shared / partially-shared / unique claims | PASS — `SharedClaims`, `PartiallySharedClaims`, `UniqueClaimsByReviewer` |
| Shared/unique debts | PASS — `compareLists` |
| Shared obstructions | PASS |
| Conflicts require opposing polarity | PASS — `polarityConflict` checks for `not`/`no`/`cannot` mixed with positive |
| Weak-grounding warnings | PASS — `PossibleHallucinations` |
| No deep-semantic claim | PASS — `AgreementScore` uses lexical Jaccard only |

## 11. Policy genericity audit

| Check | Result |
|---|---|
| Non-EBP policy triple-review | PASS — `EBP-EVAL-SIMPLE-0002` non-EBP run completed; debts are `sourceSupport`, `methodClarity`, `limitationDisclosure` |
| Debt-ID hardcoding in production code | PASS — zero `needMap|needInvariant|...` in `pkg/simple/*.go` (only in tests) |
| Generic scoring fallback | PASS — policy-driven throughout |
| Forbidden-language checks applied to report | PASS — `saveArtifacts` iterates `c.Policy.IR.ReportLanguage.ForbiddenPatterns` |
| No hardcoded EBP fallback | PASS |

## 12. Artifact and report audit

Both `/tmp/ebp-eval-simple-triple-review` and `/tmp/ebp-eval-simple-triple-review-non-ebp` contain all expected paths. `report/triple_review_report.md` includes the required limitation block verbatim, a score table, run completeness, reviewer-identity and quality separation, shared/unique claims, debt-by-reviewer, agreement/disagreement summary, weak-grounding section, recommended human review steps, and the five mandatory limitation sentences. No forbidden terms (`proved`, `solved`, `validated physics`, `full EBP promotion`, `source-faithful TreeQuest parity`, `human faithfulness completed`) appear.

## 13. Secret/privacy audit

`secretRE` in `artifacts.go:14` matches `OPENROUTER_API_KEY`, `authorization: bearer`, and `sk-` patterns. Both live bundles pass. No absolute local paths in `source_ref.json` by default (paths are relative to invocation directory). `OPENROUTER_API_KEY` string literal in `main.go:116,258` is an env var name used by `llm.NewOpenRouterClient`; it is a string constant pattern, not a value — classified as `allowed` with a note that production deployment should load via env rather than hardcoded names.

## 14. Test gaps

- Missing explicit `TestTripleReview_ReviewerIdentityNotInUserMessage` test (currently folded into `TestTripleReview_SimpleUserInstructionPreserved` with a weaker check).
- Missing `TestTripleReview_TwoCompletedOneFailed` and `TestTripleReview_OneCompletedTwoFailed` as distinct named tests in `cmd/ebp-paper-evaluator/main_test.go` (the `pkg/simple` package has one, but the CLI-level bundle path is untested for partial status).
- The `pkg/eval` TreeQuest-import boundary has no negative test that verifies the triple-review binary can be stripped of eval-linked symbols (a build-tag or `//go:build` test would catch regression).

## 15. Architecture boundary table

| Boundary | Status |
|---|---|
| `treequest-go` unchanged | PASS |
| `pkg/simple` no TreeQuest import | PASS |
| `treequest_used:false` in provenance | PASS |
| Local `.txt` / `.md` only | PASS |
| No URL/PDF/arXiv/HTTP/pdftotext | PASS |
| Identical user message | PASS |
| Reviewer identity excluded from user content | PASS (with caveat: test marginal) |
| Untrusted paper wrapper | PASS |
| Concurrent reviewer calls | PASS |
| Deterministic artifact ordering | PASS |
| 3/2/1/0 partial status handling | PASS |
| Strict JSON parsing | PASS |
| Evaluator-computed scores | PASS |
| Review quality separated from run completeness | PASS |
| Equal-weight final score | PASS |
| Lexical agreement limitation | PASS |
| Non-EBP policy fixture | PASS |
| No-faithfulness `not_assessed` | PASS |
| No proof/promotion language | PASS |
| No secret leakage | PASS |
| Triple-review binary free of TreeQuest symbols | CAVEAT — `pkg/eval` pulls `treequest-go` into `main` package; functional isolation holds but binary boundary does not |

## 16. EBP/PTW self-audit

| Debt | Status |
|---|---|
| needMap | Addressed in scoring (`MapInvariantQuality`) |
| needInvariant | Addressed in scoring |
| needToyCheck | Addressed in scoring |
| needNullModel | Addressed in scoring |
| needObstruction | Addressed in scoring (`ToyNullObstructionAwareness`) |
| needFaithfulnessReview | Explicitly not-assessed; profile sets `DebtNotAssessed`; `FaithfulnessHumility` scorer rewards `"human faithfulness review was not performed"` |
| Promotion status | `CANDIDATE_EBP_ASSESSMENT_HUMAN_REVIEW_REQUIRED`; report disclaims full EBP promotion verbatim |

## 17. Final recommended next ticket

**Ticket: EBP-EVAL-SIMPLE-0003 — Triple-Review Binary Boundary Hardening**

Scope: decouple `pkg/eval` (and therefore `treequest-go`) from the `triple-review` subcommand binary. Action items: (a) move `treequest`-using `eval` code behind a `go:build simple` or `go:build run` tag, or into an internal sub-package imported only by the `run` subcommand; (b) add `main_test.go` negative test that `strings`-scans the `triple-review` binary or confirms `main.go` contains no eval-import references; (c) strengthen `TestTripleReview_SimpleUserInstructionPreserved` to explicitly assert none of `"reviewer_1"`, `"reviewer_2"`, `"reviewer_3"` appear in any user prompt; (d) add named CLI tests for `2-of-3` and `1-of-3` partial-completeness paths. After this ticket, promote `simple_triple_review` to a stable baseline and run a real-provider smoke test against OpenRouter before declaring v0.1 release-ready.